package routing

import (
	"net/netip"
	"slices"
	"sync"

	"bjoernblessin.de/chatprotogol/sock"
	"bjoernblessin.de/chatprotogol/util/assert"
)

type Router struct {
	lsdb          map[netip.Addr]LSAEntry // Link State Database (LSDB) that holds the Link State Advertisements (LSAs) of every host (including the local LSA)
	socket        sock.Socket
	neighborTable map[netip.Addr]NeighborEntry
	routingTable  map[netip.Addr]netip.AddrPort // Maps destination IP addresses to the next hop they should use
	mu            sync.Mutex                    // Protects access to the router's state, including the LSDB, neighbor table, and routing table
}

func NewRouter(socket sock.Socket) *Router {
	return &Router{
		lsdb:          make(map[netip.Addr]LSAEntry),
		socket:        socket,
		neighborTable: make(map[netip.Addr]NeighborEntry),
		routingTable:  make(map[netip.Addr]netip.AddrPort),
	}
}

// AddNeighbor adds a new neighbor to the router.
// It adds the neighbor to the neighbor table, recalculates the local LSA, and builds the routing table.
// Asserts that the neighbor does not already exist in the neighbor table.
// Returns a slice of unreachable addresses that are safe to clear state for.
// Can be called concurrently.
func (r *Router) AddNeighbor(nextHop netip.AddrPort) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.addNeighbor(nextHop)
	localAddr := r.socket.MustGetLocalAddress().Addr()
	oldLocalLSA := r.lsdb[localAddr] // oldLocalLSA may be the zero value
	r.recalculateLocalLSA()
	notRoutable := r.buildRoutingTable()

	unreachableHosts := r.getUnreachableHosts(notRoutable, localAddr, oldLocalLSA)
	assert.Assert(len(unreachableHosts) == 0, "There should be no unreachable hosts after adding a neighbor")
}

// RemoveNeighbor removes a neighbor from the router.
// It removes the neighbor from the neighbor table, recalculates the local LSA, and builds the routing table.
// Returns a slice of unreachable addresses that are safe to clear state for.
// Can be called concurrently.
func (r *Router) RemoveNeighbor(addr netip.Addr) (unreachableHosts []netip.Addr) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.removeNeighbor(addr)
	localAddr := r.socket.MustGetLocalAddress().Addr()
	oldLocalLSA := r.lsdb[localAddr] // oldLocalLSA may be the zero value
	r.recalculateLocalLSA()
	notRoutable := r.buildRoutingTable()

	return r.getUnreachableHosts(notRoutable, localAddr, oldLocalLSA)
}

// UpdateLSA adds a new LSA to the router.
// It updates the LSA in the LSDB and builds the routing table.
// Returns a slice of unreachable addresses that are safe to clear state for.
// Can be called concurrently.
func (r *Router) UpdateLSA(srcAddr netip.Addr, seqNum uint32, neighborAddresses []netip.Addr) (unreachableHosts []netip.Addr) {
	r.mu.Lock()
	defer r.mu.Unlock()

	oldLSA := r.lsdb[srcAddr] // oldLSA may be the zero value
	r.updateLSA(srcAddr, seqNum, neighborAddresses)
	notRoutable := r.buildRoutingTable()
	return r.getUnreachableHosts(notRoutable, srcAddr, oldLSA)
}

// getUnreachableHosts gets all hosts that are no longer reachable.
// Unreachable hosts are those that are not routable anymore (but where previously), i.e., they are not in the routing table and are affected by the LSA update that caused this function to be called.
// Unreachable hosts is always a subset of notRoutableHosts.
// This function
//  1. Checks if the LSA update removed a neighbor relationship.
//  2. If so, it checks if the other host (lsaOwner's previous neighbor) is still reachable.
//  3. If not, it collects all hosts that are not routable anymore and clears their state.
//
// This function is called after an LSA update.
func (r *Router) getUnreachableHosts(notRoutableHosts []netip.Addr, lsaOwner netip.Addr, oldLSA LSAEntry) (unreachableHosts []netip.Addr) {
	currentLSA, exists := r.lsdb[lsaOwner]
	assert.Assert(exists, "LSA for %v not found in LSDB", lsaOwner)

	if len(currentLSA.Neighbors) >= len(oldLSA.Neighbors) {
		// No neighbors were removed, so no hosts are unreachable
		return nil
	}

	// Determine which neighbor was removed
	var removedNeighbor netip.Addr
	for _, oldNeighbor := range oldLSA.Neighbors {
		if !slices.Contains(currentLSA.Neighbors, oldNeighbor) {
			removedNeighbor = oldNeighbor
			break
		}
	}

	// Check if the removed neighbor is still reachable
	_, exists = r.routingTable[removedNeighbor]
	if exists || removedNeighbor == r.socket.MustGetLocalAddress().Addr() { // We aren't "routable" but still considered reachable
		// The removed neighbor is still routable, so no hosts are unreachable
		return nil
	}

	// BFS to find all unreachable hosts
	unreachableHosts = make([]netip.Addr, 0, len(notRoutableHosts))

	removedNeighborLSA, ok := r.lsdb[removedNeighbor]
	if !ok {
		return nil // If the removed neighbor's LSA is not in the LSDB, it's not  considered unreachable
	}

	removedNeighborNeighbors := removedNeighborLSA.Neighbors
	removedNeighborNeighbors = slices.DeleteFunc(removedNeighborNeighbors, func(addr netip.Addr) bool {
		return addr == lsaOwner
	})

	visited := make(map[netip.Addr]bool)
	queue := []netip.Addr{}

	visited[removedNeighbor] = true
	assert.Assert(len(unreachableHosts) < len(notRoutableHosts), "Unreachable hosts slice should not exceed notRoutableHosts length")
	unreachableHosts = append(unreachableHosts, removedNeighbor)

	// Start BFS from all neighbors of the removed neighbor (excluding the lsaOwner)
	for _, neighbor := range removedNeighborNeighbors {
		queue = append(queue, neighbor)
	}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		if visited[node] {
			continue
		}

		lsa, ok := r.lsdb[node]
		if !ok {
			// If the node's LSA is not in the LSDB, it means we encountered this node only via a neighbor that has this node in it's LSA.
			// We don't have this node's LSA, so we don't consider it unreachable.
			continue
		}

		visited[node] = true
		assert.Assert(len(unreachableHosts) < len(notRoutableHosts), "Unreachable hosts slice should not exceed notRoutableHosts length")
		unreachableHosts = append(unreachableHosts, node)

		// Enqueue neighbors of this node from LSDB
		for _, n := range lsa.Neighbors {
			if !visited[n] {
				queue = append(queue, n)
			}
		}
	}

	return unreachableHosts
}

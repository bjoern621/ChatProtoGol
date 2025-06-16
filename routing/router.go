package routing

import (
	"net/netip"
	"sync"

	"bjoernblessin.de/chatprotogol/sock"
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
// Can be called concurrently.
func (r *Router) AddNeighbor(nextHop netip.AddrPort) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.addNeighbor(nextHop)
	r.recalculateLocalLSA()
	_ = r.buildRoutingTable() // there should be no unreachable hosts
}

// RemoveNeighbor removes a neighbor from the router.
// It removes the neighbor from the neighbor table, recalculates the local LSA, and builds the routing table.
// Returns a slice of unreachable addresses that could not be reached during the routing table build process.
// Can be called concurrently.
func (r *Router) RemoveNeighbor(addr netip.Addr) (unreachableAddrs []netip.Addr) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.removeNeighbor(addr)
	r.recalculateLocalLSA()
	unreachableHosts := r.buildRoutingTable()
	return unreachableHosts
}

// UpdateLSA adds a new LSA to the router.
// It updates the LSA in the LSDB and builds the routing table.
// Returns a slice of unreachable addresses that could not be reached during the routing table build process.
// Can be called concurrently.
func (r *Router) UpdateLSA(srcAddr netip.Addr, seqNum uint32, neighborAddresses []netip.Addr) (unreachableAddrs []netip.Addr) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.updateLSA(srcAddr, seqNum, neighborAddresses)
	unreachableHosts := r.buildRoutingTable()
	return unreachableHosts
}

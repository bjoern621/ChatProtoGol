package routing

import (
	"container/heap"
	"math"
	"net/netip"

	"bjoernblessin.de/chatprotogol/util/assert"
)

type LSAEntry struct {
	SeqNum    uint32 // The sequence number ("version") of the LSA
	Neighbors []netip.Addr
}

// recalculateLocalLSA recalculates the local LSA.
// The sequence number is incremented for the local address.
func (r *Router) recalculateLocalLSA() {
	localAddr := r.socket.MustGetLocalAddress().Addr()

	localLSA := LSAEntry{
		SeqNum:    r.getNextSequenceNumber(localAddr),
		Neighbors: make([]netip.Addr, 0, len(r.neighborTable)),
	}

	for neighborAddr := range r.neighborTable {
		localLSA.Neighbors = append(localLSA.Neighbors, neighborAddr)
	}

	r.lsdb[localAddr] = localLSA
}

// getNextSequenceNumber returns the next sequence number for the given address.
// If the address does not exist in the LSDB, it returns 0 as the default sequence number.
func (r *Router) getNextSequenceNumber(addr netip.Addr) uint32 {
	if entry, exists := r.lsdb[addr]; exists {
		return entry.SeqNum + 1
	}
	return 0 // Default sequence number if not found
}

func (r *Router) GetLSA(addr netip.Addr) (LSAEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if entry, exists := r.lsdb[addr]; exists {
		return entry, true
	}
	return LSAEntry{}, false
}

// GetAvailableLSAs returns a slice of all available LSAs in the LSDB.
func (r *Router) GetAvailableLSAs() []netip.Addr {
	r.mu.Lock()
	defer r.mu.Unlock()

	addresses := make([]netip.Addr, 0, len(r.lsdb))
	for addr := range r.lsdb {
		addresses = append(addresses, addr)
	}
	return addresses
}

type DijkstraNode struct {
	Addr    netip.Addr
	NextHop *netip.AddrPort
	Dist    int // Distance from the source node
	index   int // Index in the priority queue for heap operations
}

type dijkstraPriorityQueue []*DijkstraNode

func (pq dijkstraPriorityQueue) Len() int { return len(pq) }

func (pq dijkstraPriorityQueue) Less(i, j int) bool {
	return pq[i].Dist < pq[j].Dist // Min-heap based on distance
}

func (pq dijkstraPriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *dijkstraPriorityQueue) Push(x any) {
	node := x.(*DijkstraNode)
	node.index = len(*pq)
	*pq = append(*pq, node)
}

func (pq *dijkstraPriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[:n-1]
	return item
}

func (pq *dijkstraPriorityQueue) update(node *DijkstraNode, newDist int, nextHop *netip.AddrPort) {
	node.Dist = newDist
	node.NextHop = nextHop
	heap.Fix(pq, node.index)
}

// Creates the current topology of the network based on the LSAs in the LSDB.
// Runs the Dijkstra algorithm to calculate the shortest paths and build the routing table.
func (r *Router) buildRoutingTable() {
	assert.Assert(len(r.lsdb) > 0, "LSDB must not be empty to build the routing table")

	queue := make(dijkstraPriorityQueue, len(r.lsdb)-1)
	i := 0
	localAddr := r.socket.MustGetLocalAddress().Addr()
	for addr := range r.lsdb {
		if addr == localAddr {
			continue // Skip the local address, as it does not need a routing entry
		}

		var nextHop *netip.AddrPort
		var dist int
		isNeighbor, addrPort := r.IsNeighbor(addr)
		if isNeighbor {
			nextHop = &addrPort
			dist = 1 // Direct neighbors have a distance of 1
		} else {
			nextHop = nil
			dist = math.MaxInt // Non-neighbors are initially unreachable
		}

		queue[i] = &DijkstraNode{
			Addr:    addr,
			NextHop: nextHop,
			Dist:    dist,
		}
		i++
	}

	// Add neighbors we don't have in the LSDB yet. This is useful when a new neighbor connects and we want to ensure it is reachable.
	for neighborAddr, neighborEntry := range r.neighborTable {
		if _, exists := r.lsdb[neighborAddr]; !exists {
			queue = append(queue, &DijkstraNode{
				Addr:    neighborAddr,
				NextHop: &neighborEntry.NextHop,
				Dist:    1, // Neighbors are reachable with a distance of 1
			})
		}
	}

	heap.Init(&queue)

	r.routingTable = make(map[netip.Addr]netip.AddrPort, len(queue))

	for queue.Len() > 0 {
		currentNode := heap.Pop(&queue).(*DijkstraNode)

		if currentNode.Dist == math.MaxInt {
			break // All remaining nodes are unreachable
		}

		r.routingTable[currentNode.Addr] = *currentNode.NextHop

		// Update the distance of adjacent nodes that are still unvisited (not in the routing table and not the local address)
		for _, neighborAddr := range r.lsdb[currentNode.Addr].Neighbors {
			if _, exists := r.routingTable[neighborAddr]; exists {
				continue // Skip if the neighbor is already in the routing table
			}
			if neighborAddr == localAddr {
				continue // Skip the local address
			}

			// Find the corresponding node in the queue for the neighbor
			var neighborNode *DijkstraNode
			for i := range queue.Len() { // TODO optimize using map
				if queue[i].Addr == neighborAddr {
					neighborNode = queue[i]
					break
				}
			}

			assert.IsNotNil(neighborNode, "Neighbor node should exist in the queue")

			// Update the neighbor if a shorter path is found
			if currentNode.Dist+1 < neighborNode.Dist {
				queue.update(neighborNode, currentNode.Dist+1, currentNode.NextHop)
			}
		}
	}
}

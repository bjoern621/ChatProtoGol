package routing

import (
	"container/heap"
	"math"
	"net/netip"

	"bjoernblessin.de/chatprotogol/util/assert"
)

func (r *Router) GetNextHop(destinationIP netip.Addr) (addrPort netip.AddrPort, found bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.routingTable[destinationIP]
	if !exists {
		return netip.AddrPort{}, false
	}

	return entry, true
}

// GetRoutingTable returns the current routing table entries.
func (r *Router) GetRoutingTable() map[netip.Addr]netip.AddrPort {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.routingTable
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
// Returns a slice of unreachable addresses that could not be reached during the routing table build process.
func (r *Router) buildRoutingTable() (unreachableAddrs []netip.Addr) {
	assert.Assert(len(r.lsdb) > 0, "LSDB must not be empty to build the routing table")

	queue := make(dijkstraPriorityQueue, 0, len(r.lsdb)) // Can't be len(r.lsdb-1) because we might not have our local LSA yet but just received a new neighbor's LSA.
	localAddr := r.socket.MustGetLocalAddress().Addr()
	for addr := range r.lsdb {
		if addr == localAddr {
			continue // Skip the local address, as it does not need a routing entry
		}

		var nextHop *netip.AddrPort
		var dist int
		isNeighbor, addrPort := r.isNeighbor(addr)
		if isNeighbor {
			nextHop = &addrPort
			dist = 1 // Direct neighbors have a distance of 1
		} else {
			nextHop = nil
			dist = math.MaxInt // Non-neighbors are initially unreachable
		}

		queue = append(queue, &DijkstraNode{
			Addr:    addr,
			NextHop: nextHop,
			Dist:    dist,
		})
	}

	heap.Init(&queue)

	r.routingTable = make(map[netip.Addr]netip.AddrPort, len(queue))
	unreachableAddrs = make([]netip.Addr, 0)

	for queue.Len() > 0 {
		currentNode := heap.Pop(&queue).(*DijkstraNode)

		if currentNode.Dist == math.MaxInt {
			// All remaining nodes are unreachable
			unreachableAddrs = append(unreachableAddrs, currentNode.Addr)
			continue
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

			if neighborNode == nil {
				// If the neighbor is not in the queue, it means it's LSA is not present (yet) but it's a neighbor of another node where we have the LSA.
				// We don't add here, the neighbor is considered unreachable for now.
				continue
			}

			// Update the neighbor if a shorter path is found
			if currentNode.Dist+1 < neighborNode.Dist {
				queue.update(neighborNode, currentNode.Dist+1, currentNode.NextHop)
			}
		}
	}

	return unreachableAddrs
}

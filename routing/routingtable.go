package routing

import (
	"container/heap"
	"net/netip"
)

func (r *Router) GetNextHop(destinationIP netip.Addr) (addrPort netip.AddrPort, found bool) {
	entry, exists := r.routingTable[destinationIP]
	if !exists {
		return netip.AddrPort{}, false
	}

	return entry, true
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

package routing

import (
	"container/heap"
	"math"
	"net/netip"

	"bjoernblessin.de/chatprotogol/sock"
	"bjoernblessin.de/chatprotogol/util/assert"
)

type LSAEntry struct {
	seqNum    [4]byte // The sequence number ("version") of the LSA
	neighbors []netip.Addr
}

func (r *Router) RecalculateLocalLSA() {
	localAddr := r.socket.MustGetLocalAddress().Addr()

	localLSA := LSAEntry{
		seqNum:    r.getLatestSequenceNumber(localAddr),
		neighbors: make([]netip.Addr, 0, len(r.neighborTable)),
	}

	for neighborAddr := range r.neighborTable {
		localLSA.neighbors = append(localLSA.neighbors, neighborAddr)
	}

	r.lsdb[localAddr] = localLSA
}

func (r *Router) getLatestSequenceNumber(addr netip.Addr) [4]byte {
	if entry, exists := r.lsdb[addr]; exists {
		return entry.seqNum
	}
	return [4]byte{0, 0, 0, 0} // Default sequence number if not found
}

func (r *Router) GetLSA(addr netip.Addr) (LSAEntry, bool) {
	if entry, exists := r.lsdb[addr]; exists {
		return entry, true
	}
	return LSAEntry{}, false
}

// Creates the current topology of the network based on the LSAs in the LSDB.
// Runs the Dijkstra algorithm to calculate the shortest paths and build the routing table.
func (r *Router) BuildRoutingTable(socket sock.Socket) {
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
		for _, neighborAddr := range r.lsdb[currentNode.Addr].neighbors {
			if _, exists := r.routingTable[neighborAddr]; exists {
				continue // Skip if the neighbor is already in the routing table
			}
			if neighborAddr == localAddr {
				continue // Skip the local address
			}

			// Find the corresponding node in the queue for the neighbor
			var neighborNode *DijkstraNode
			for i := range queue.Len() {
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

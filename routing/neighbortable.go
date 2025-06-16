package routing

import (
	"net/netip"

	"bjoernblessin.de/chatprotogol/util/assert"
)

// NeighborEntry represents a neighbor in the neighbor table.
type NeighborEntry struct {
	NextHop netip.AddrPort
}

// addNeighbor adds a new neighbor to the neighbor table.
// It checks if the neighbor already exists and asserts that it does not.
func (r *Router) addNeighbor(nextHop netip.AddrPort) {
	_, exists := r.neighborTable[nextHop.Addr()]
	assert.Assert(!exists, "Neighbor already exists in the neighbor table: %s", nextHop.Addr().String())

	r.neighborTable[nextHop.Addr()] = NeighborEntry{NextHop: nextHop}
}

// IsNeighbor checks if the given address is a neighbor.
// It returns a boolean indicating if the address is a neighbor and if so, the address and port for that neighbor.
// Can be called concurrently.
func (r *Router) IsNeighbor(addr netip.Addr) (bool, netip.AddrPort) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.isNeighbor(addr)
}

func (r *Router) isNeighbor(addr netip.Addr) (bool, netip.AddrPort) {
	entry, exists := r.neighborTable[addr]
	if !exists {
		return false, netip.AddrPort{}
	}
	return true, entry.NextHop
}

// Can be called concurrently.
func (r *Router) GetNeighbors() map[netip.Addr]netip.AddrPort {
	r.mu.Lock()
	defer r.mu.Unlock()

	neighbors := make(map[netip.Addr]netip.AddrPort, len(r.neighborTable))
	for addr, entry := range r.neighborTable {
		neighbors[addr] = entry.NextHop
	}
	return neighbors
}

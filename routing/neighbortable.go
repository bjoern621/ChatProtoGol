package routing

import (
	"net/netip"

	"bjoernblessin.de/chatprotogol/util/assert"
)

// NeighborEntry represents a neighbor in the neighbor table.
type NeighborEntry struct {
	NextHop netip.AddrPort
}

// AddNeighbor adds a new neighbor to the neighbor table.
// It checks if the neighbor already exists and asserts that it does not.
func (r *Router) AddNeighbor(nextHop netip.AddrPort) {
	_, exists := r.NeighborTable[nextHop.Addr()]
	assert.Assert(!exists, "Neighbor already exists in the neighbor table: %s", nextHop.Addr().String())

	r.NeighborTable[nextHop.Addr()] = NeighborEntry{NextHop: nextHop}
}

// IsNeighbor checks if the given address is a neighbor.
// It returns a boolean indicating if the address is a neighbor and if so, the address and port for that neighbor.
func (r *Router) IsNeighbor(addr netip.Addr) (bool, netip.AddrPort) {
	entry, exists := r.NeighborTable[addr]
	if !exists {
		return false, netip.AddrPort{}
	}
	return true, entry.NextHop
}

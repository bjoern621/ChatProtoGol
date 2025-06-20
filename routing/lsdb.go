package routing

import (
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

// updateLSA adds a new LSA to the LSDB.
// Asserts that the sequence number is greater than any existing LSA for the same address.
func (r *Router) updateLSA(addr netip.Addr, seqNum uint32, neighbors []netip.Addr) {
	existingLSA, exists := r.lsdb[addr]
	assert.Assert(!(exists && existingLSA.SeqNum >= seqNum), "Cannot add LSA with older or equal sequence number")

	r.lsdb[addr] = LSAEntry{
		SeqNum:    seqNum,
		Neighbors: neighbors,
	}
}

// getNextSequenceNumber returns the next sequence number for the given address's LSA.
// If the address does not exist in the LSDB, it returns 0 as the default sequence number.
func (r *Router) getNextSequenceNumber(addr netip.Addr) uint32 {
	if entry, exists := r.lsdb[addr]; exists {
		return entry.SeqNum + 1
	}
	return 0 // Default sequence number if not found
}

// Can be called concurrently.
func (r *Router) GetLSA(addr netip.Addr) (LSAEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if entry, exists := r.lsdb[addr]; exists {
		return entry, true
	}
	return LSAEntry{}, false
}

// RemoveLSA removes an LSA from the LSDB.
// Can be called concurrently.
// It does not affect the routing table directly, SHOULD BE CALLED AFTER GETTING unreachableHosts FROM an routing table update.
func (r *Router) RemoveLSA(addr netip.Addr) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.lsdb, addr)
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

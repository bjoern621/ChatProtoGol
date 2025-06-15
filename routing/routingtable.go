package routing

import (
	"net/netip"
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

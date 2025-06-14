package routing

import (
	"net/netip"
)

func (r *Router) GetNextHop(destinationIP netip.Addr) (addrPort netip.AddrPort, found bool) {
	entry, exists := r.routingTable[destinationIP]
	if !exists {
		return netip.AddrPort{}, false
	}

	return entry, true
}

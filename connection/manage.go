package connection

import "net/netip"

// ClearUnreachableHosts clears state for hosts that are no longer reachable.
// This includes removing their LSAs from the LSDB, their sequencing state and their payload buffer in the reconstruction package.
// May be called with the zero list in which case it does nothing.
func ClearUnreachableHosts(unreachableHosts []netip.Addr) {
	for _, addr := range unreachableHosts {
		router.RemoveLSA(addr)
		incomingSequencing.ClearIncomingPacketNumbers(addr)
		outgoingSequencing.ClearPacketNumbers(addr)
		reconstructor.ClearPayloadBuffer(addr)
	}
}

package connection

import (
	"net/netip"

	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/sequencing/reconstruction"
	"bjoernblessin.de/chatprotogol/util/logger"
)

// ClearUnreachableHosts clears state for hosts that are no longer reachable.
// This includes removing their LSAs from the LSDB, their sequencing state and their payload buffer in the reconstruction package.
// May be called with the zero list in which case it does nothing.
func ClearUnreachableHosts(unreachableHosts []netip.Addr) {
	for _, addr := range unreachableHosts {
		logger.Infof("Clearing unreachable host %s", addr)
		router.RemoveLSA(addr)
		incomingSequencing.ClearIncomingPacketNumbers(addr)
		outgoingSequencing.ClearPacketNumbers(addr)
		sequencing.ClearBlockers(addr)
		reconstruction.ClearFileReconstructor(addr)
		reconstruction.ClearMsgReconstructor(addr)
	}
}

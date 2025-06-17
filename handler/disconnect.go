package handler

import (
	"net/netip"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/routing"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/sock"
	"bjoernblessin.de/chatprotogol/util/assert"
	"bjoernblessin.de/chatprotogol/util/logger"
)

// handleDisconnect processes a disconnect request from a peer.
func handleDisconnect(packet *pkt.Packet, inSequencing *sequencing.IncomingPktNumHandler, router *routing.Router, socket sock.Socket) {
	duplicate, dupErr := inSequencing.IsDuplicatePacket(packet)
	if dupErr != nil {
		logger.Warnf(dupErr.Error())
		return
	} else if duplicate {
		_ = connection.SendRoutedAcknowledgment(netip.AddrFrom4(packet.Header.SourceAddr), packet.Header.PktNum)
		return
	}

	logger.Infof("DISCO FROM %v %v", packet.Header.SourceAddr, packet.Header.PktNum)

	srcAddr := netip.AddrFrom4(packet.Header.SourceAddr)

	if isNeighbor, _ := router.IsNeighbor(srcAddr); !isNeighbor {
		logger.Warnf("Received disconnect from non-neighbor peer %v", srcAddr)
		return
	}

	_ = connection.SendRoutedAcknowledgment(srcAddr, packet.Header.PktNum)

	unreachableHosts := router.RemoveNeighbor(srcAddr)
	connection.ClearUnreachableHosts(unreachableHosts)

	localAddr := socket.MustGetLocalAddress().Addr()
	localLSA, exists := router.GetLSA(localAddr)
	assert.Assert(exists, "Local LSA should exist for the local address")
	connection.FloodLSA(localAddr, localLSA)
}

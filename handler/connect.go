package handler

import (
	"net"
	"net/netip"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/routing"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/sock"
	"bjoernblessin.de/chatprotogol/util/assert"
	"bjoernblessin.de/chatprotogol/util/logger"
)

// handleConnect processes a connection request from a peer.
func handleConnect(packet *pkt.Packet, sourceAddr *net.UDPAddr, router *routing.Router, inSequencing *sequencing.IncomingPktNumHandler, socket sock.Socket) {
	duplicate, dupErr := inSequencing.IsDuplicatePacket(packet)
	if dupErr != nil {
		return
	} else if duplicate {
		_ = connection.SendRoutedAcknowledgment(netip.AddrFrom4(packet.Header.SourceAddr), packet.Header.PktNum)
		return
	}

	logger.Infof("CONN FROM %v %v", packet.Header.SourceAddr, packet.Header.PktNum)

	router.AddNeighbor(sourceAddr.AddrPort())

	_ = connection.SendRoutedAcknowledgment(sourceAddr.AddrPort().Addr(), packet.Header.PktNum)

	localAddr := socket.MustGetLocalAddress().Addr()
	localLSA, exists := router.GetLSA(localAddr)
	assert.Assert(exists, "Local LSA should exist for the local address")
	connection.FloodLSA(localAddr, localLSA)

	err := connection.SendDD(sourceAddr.AddrPort().Addr())
	if err != nil {
		logger.Warnf("Failed to send database description to %s: %v", sourceAddr.AddrPort(), err)
	}
}

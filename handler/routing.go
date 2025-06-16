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

// handleRoutingDuplicate checks if the packet is a duplicate.
// If it is, it sends an acknowledgment back to the sender.
// Returns true if the packet was handled (duplicate or errornous packet), false otherwise.
func handleRoutingDuplicate(packet *pkt.Packet, inSequencing *sequencing.IncomingPktNumHandler) (handled bool) {
	duplicate, dupErr := inSequencing.IsDuplicatePacket(packet)
	if dupErr != nil {
		return true
	} else if duplicate {
		_ = connection.SendRoutedAcknowledgment(netip.AddrFrom4(packet.Header.SourceAddr), packet.Header.PktNum)
		return true
	}

	return false
}

// handleConnect processes a connection request from a peer.
func handleConnect(packet *pkt.Packet, sourceAddr *net.UDPAddr, router *routing.Router, inSequencing *sequencing.IncomingPktNumHandler, socket sock.Socket) {
	handled := handleRoutingDuplicate(packet, inSequencing)
	if handled {
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

// handleDisconnect processes a disconnect request from a peer.
func handleDisconnect(packet *pkt.Packet, inSequencing *sequencing.IncomingPktNumHandler, router *routing.Router, socket sock.Socket) {
	handled := handleRoutingDuplicate(packet, inSequencing)
	if handled {
		return
	}

	logger.Infof("DISCO FROM %v %v", packet.Header.SourceAddr, packet.Header.PktNum)

	srcAddr := netip.AddrFrom4(packet.Header.SourceAddr)

	if isNeighbor, _ := router.IsNeighbor(srcAddr); !isNeighbor {
		logger.Warnf("Received disconnect from non-neighbor peer %v", srcAddr)
		return
	}

	_ = connection.SendRoutedAcknowledgment(srcAddr, packet.Header.PktNum)

	router.RemoveNeighbor(srcAddr)

	localAddr := socket.MustGetLocalAddress().Addr()
	localLSA, exists := router.GetLSA(localAddr)
	assert.Assert(exists, "Local LSA should exist for the local address")
	connection.FloodLSA(localAddr, localLSA)
}

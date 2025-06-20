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
func handleDisconnect(packet *pkt.Packet, inSequencing *sequencing.IncomingPktNumHandler, router *routing.Router, socket sock.Socket, srcAddrPort netip.AddrPort) {
	duplicate, dupErr := inSequencing.IsDuplicatePacket(packet)
	if dupErr != nil {
		logger.Warnf(dupErr.Error())
		return
	} else if duplicate {
		_ = connection.SendAcknowledgmentTo(srcAddrPort, packet.Header.PktNum)
		return
	}

	logger.Infof("DISCO FROM %v %v", packet.Header.SourceAddr, packet.Header.PktNum)

	srcAddr := netip.AddrFrom4(packet.Header.SourceAddr)
	if srcAddr != srcAddrPort.Addr() {
		logger.Warnf("Malformed CON packet: source address %v does not match sender %v", srcAddr, srcAddrPort)
		return
	}

	destAddr := netip.AddrFrom4(packet.Header.DestAddr)
	localAddr := socket.MustGetLocalAddress().Addr()
	if destAddr != localAddr {
		logger.Warnf("Malformed DIS packet: destination address %v does not match local address %v", destAddr, localAddr)
		return
	}

	if isNeighbor, _ := router.IsNeighbor(srcAddr); !isNeighbor {
		logger.Warnf("Received disconnect from non-neighbor peer %v", srcAddr)
		return
	}

	_ = connection.SendAcknowledgmentTo(srcAddrPort, packet.Header.PktNum)

	unreachableHosts := router.RemoveNeighbor(srcAddr)
	connection.ClearUnreachableHosts(unreachableHosts)

	localLSA, exists := router.GetLSA(localAddr)
	assert.Assert(exists, "Local LSA should exist for the local address")
	connection.FloodLSA(localAddr, localLSA)
}

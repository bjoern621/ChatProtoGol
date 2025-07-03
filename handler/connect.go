package handler

import (
	"fmt"
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
func handleConnect(packet *pkt.Packet, srcAddrPort netip.AddrPort, router *routing.Router, inSequencing *sequencing.IncomingPktNumHandler, socket sock.Socket) {
	duplicate, dupErr := inSequencing.IsDuplicatePacket(packet)
	if dupErr != nil {
		logger.Warnf(dupErr.Error())
		return
	} else if duplicate {
		_ = connection.SendAcknowledgmentTo(srcAddrPort, packet.Header.PktNum)
		return
	}

	logger.Debugf("CONN FROM %v %v", packet.Header.SourceAddr, packet.Header.PktNum)

	srcAddr := netip.AddrFrom4(packet.Header.SourceAddr)
	if srcAddr != srcAddrPort.Addr() {
		logger.Warnf("Malformed CON packet: source address %v does not match sender %v", srcAddr, srcAddrPort)
		return
	}

	destAddr := netip.AddrFrom4(packet.Header.DestAddr)
	localAddr := socket.MustGetLocalAddress().Addr()
	if destAddr != localAddr {
		logger.Warnf("Malformed CON packet: destination address %v does not match local address %v", destAddr, localAddr)
		return
	}

	if isNeighbor, _ := router.IsNeighbor(srcAddr); isNeighbor {
		logger.Warnf("Received connection request from already known neighbor %v", srcAddr)
		return
	}

	// Valid packet

	_ = connection.SendAcknowledgmentTo(srcAddrPort, packet.Header.PktNum)

	router.AddNeighbor(srcAddrPort)

	localLSA, exists := router.GetLSA(localAddr)
	assert.Assert(exists, "Local LSA should exist for the local address")
	connection.FloodLSA(localAddr, localLSA)

	err := connection.SendDD(srcAddrPort)
	if err != nil {
		logger.Warnf("Failed to send database description to %s: %v", srcAddrPort, err)
	}

	fmt.Printf("Connected to %s\n", srcAddrPort)
}

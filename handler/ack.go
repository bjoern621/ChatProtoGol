package handler

import (
	"net/netip"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/util/logger"
)

func handleAck(packet *pkt.Packet) {
	logger.Infof("ACK RECEIVED %v %d", packet.Header.SourceAddr, packet.Header.SeqNum)

	addr := netip.AddrFrom4([4]byte(packet.Header.SourceAddr))
	peer, exists := connection.GetPeer(addr)
	if !exists {
		logger.Warnf("Received ACK for unknown peer %v", addr)
		return
	}

	connection.RemoveOpenAck(peer, packet.Header.SeqNum)
}

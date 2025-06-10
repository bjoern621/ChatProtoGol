package handler

import (
	"net/netip"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/util/logger"
)

func handleAck(packet *pkt.Packet) {
	logger.Infof("ACK RECEIVED %v %d", packet.Header.SourceAddr, packet.Header.SeqNum)

	addr := netip.AddrFrom4([4]byte(packet.Header.SourceAddr))
	peer, exists := connection.GetPeer(addr)
	if !exists {
		// Peer was already removed or never existed
		// e.g. we send an disconnect message (and removed the corresponding peer) and receive their ACK
		return
	}

	sequencing.RemoveOpenAck(peer.Address, packet.Header.SeqNum)
}

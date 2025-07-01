package handler

import (
	"net/netip"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/sock"
	"bjoernblessin.de/chatprotogol/util/logger"
)

func handleAck(packet *pkt.Packet, socket sock.Socket, outSequencing *sequencing.OutgoingPktNumHandler) {
	logger.Debugf("ACK RECEIVED %v %d", packet.Header.SourceAddr, packet.Header.PktNum)

	destAddr := netip.AddrFrom4(packet.Header.DestAddr)
	if destAddr != socket.MustGetLocalAddress().Addr() {
		// The acknowledgment is for another peer, forward it

		connection.ForwardRouted(packet)
		return
	}

	// The acknowledgment is for us, remove the open acknowledgment

	srcAddr := netip.AddrFrom4([4]byte(packet.Header.SourceAddr))
	outSequencing.RemoveOpenAck(srcAddr, packet.Header.PktNum)
}

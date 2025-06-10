package handler

import (
	"net/netip"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/socket"
	"bjoernblessin.de/chatprotogol/util/logger"
)

func handleAck(packet *pkt.Packet) {
	logger.Infof("ACK RECEIVED %v %d", packet.Header.SourceAddr, packet.Header.SeqNum)

	destAddr := netip.AddrFrom4(packet.Header.DestAddr)
	if destAddr != socket.GetLocalAddress().AddrPort().Addr() {
		// The acknowledgment is for another peer, forward it

		destPeer, found := connection.GetPeer(destAddr)
		if !found {
			logger.Warnf("No peer found for destination address %s, can't forward", destAddr)
			return
		}

		destPeer.Forward(packet)
		return
	}

	// The acknowledgment is for us, remove the open acknowledgment

	sourceAddr := netip.AddrFrom4([4]byte(packet.Header.SourceAddr))
	sequencing.RemoveOpenAck(sourceAddr, packet.Header.SeqNum)
}

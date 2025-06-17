package handler

import (
	"net/netip"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/sock"
	"bjoernblessin.de/chatprotogol/util/logger"
)

func handleMsg(packet *pkt.Packet, socket sock.Socket, inSequencing *sequencing.IncomingPktNumHandler) {
	logger.Infof("MSG RECEIVED %v %d", packet.Header.SourceAddr, packet.Header.PktNum)

	destAddr := netip.AddrFrom4(packet.Header.DestAddr)

	if destAddr != socket.MustGetLocalAddress().Addr() {
		// The message is for another peer

		connection.ForwardRouted(packet)
		return
	}

	// The message is for us

	duplicate, dupErr := inSequencing.IsDuplicatePacket(packet)
	if dupErr != nil {
		logger.Warnf(dupErr.Error())
		return
	} else if duplicate {
		_ = connection.SendRoutedAcknowledgment(netip.AddrFrom4(packet.Header.SourceAddr), packet.Header.PktNum)
		return
	}

	srcAddr := netip.AddrFrom4(packet.Header.SourceAddr)

	_ = connection.SendRoutedAcknowledgment(srcAddr, packet.Header.PktNum)
}

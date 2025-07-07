package handler

import (
	"net/netip"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/sequencing/reconstruction"
	"bjoernblessin.de/chatprotogol/sock"
	"bjoernblessin.de/chatprotogol/util/logger"
)

func handleFileTransfer(packet *pkt.Packet, socket sock.Socket, inSequencing *sequencing.IncomingPktNumHandler) {
	logger.Tracef("FILE RECEIVED %v %d", packet.Header.SourceAddr, packet.Header.PktNum)

	destAddr := netip.AddrFrom4(packet.Header.DestAddr)

	if destAddr != socket.MustGetLocalAddress().Addr() {
		// The file transfer is for another peer
		connection.ForwardRouted(packet)
		return
	}

	// The file transfer is for us

	srcAddr := netip.AddrFrom4(packet.Header.SourceAddr)

	duplicate, dupErr := inSequencing.IsDuplicatePacket(packet) // TODO what if received packet twice really fast -> second is set as duplicate, and then a fin is send, even though we aren't ready for a fin
	if dupErr != nil {
		logger.Warnf(dupErr.Error())
		return
	} else if duplicate {
		_ = connection.SendRoutedAcknowledgment(srcAddr, packet.Header.PktNum)
		return
	}

	reconstruction.GetFileReconstructor(srcAddr).HandleIncomingFilePacket(packet)

	_ = connection.SendRoutedAcknowledgment(srcAddr, packet.Header.PktNum)
}

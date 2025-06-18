package handler

import (
	"fmt"
	"net/netip"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/sequencing/reconstruction"
	"bjoernblessin.de/chatprotogol/util/logger"
)

func handleFinish(packet *pkt.Packet, inSequencing *sequencing.IncomingPktNumHandler, reconstructor *reconstruction.PktSequenceReconstructor) {
	duplicate, dupErr := inSequencing.IsDuplicatePacket(packet)
	if dupErr != nil {
		logger.Warnf(dupErr.Error())
		return
	} else if duplicate {
		_ = connection.SendRoutedAcknowledgment(netip.AddrFrom4(packet.Header.SourceAddr), packet.Header.PktNum)
		return
	}

	logger.Infof("FINISH FROM %v %d", packet.Header.SourceAddr, packet.Header.PktNum)

	addr := netip.AddrFrom4(packet.Header.SourceAddr)

	_ = connection.SendRoutedAcknowledgment(addr, packet.Header.PktNum)

	completeMsg, err := reconstructor.FinishPacketSequence(addr)
	if err != nil {
		logger.Warnf("Failed to finish packet sequence: %v", err)
		return
	}

	fmt.Printf("MSG %v: %s\n", addr, completeMsg)
}

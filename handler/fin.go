package handler

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/sequencing/reconstruction"
	"bjoernblessin.de/chatprotogol/sock"
	"bjoernblessin.de/chatprotogol/util/logger"
)

func handleFinish(packet *pkt.Packet, inSequencing *sequencing.IncomingPktNumHandler, socket sock.Socket) {
	logger.Tracef("FINISH FROM %v %d", packet.Header.SourceAddr, packet.Header.PktNum)

	destAddr := netip.AddrFrom4(packet.Header.DestAddr)

	if destAddr != socket.MustGetLocalAddress().Addr() {
		// The message is for another peer
		connection.ForwardRouted(packet)
		return
	}

	// The message is for us

	if len(packet.Payload) < 4 {
		logger.Warnf("Received FINISH packet with insufficient payload length from %v", packet.Header.SourceAddr)
		return
	}

	lastPktNum := binary.BigEndian.Uint32(packet.Payload[:4])

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

	fileReconstructor, exists := reconstruction.GetFileReconstructor(srcAddr)
	if exists {
		highestFilePktNum, err := fileReconstructor.GetHighestPktNum()
		if err == nil && highestFilePktNum == lastPktNum {
			// This is a file transfer completion packet

			logger.Infof("File transfer completed for %v", srcAddr)

			filePath, err := fileReconstructor.FinishFilePacketSequence()
			if err != nil {
				logger.Warnf("Failed to finish file packet sequence: %v", err)
			}

			reconstruction.ClearFileReconstructor(srcAddr)

			fmt.Printf("FILE %v: %s\n", srcAddr, filePath)
			return
		}
	}

	msgReconstructor, exists := reconstruction.GetMsgReconstructor(srcAddr)
	if exists {
		highestMsgPktNum, err := msgReconstructor.GetHighestPktNum()
		if err == nil && highestMsgPktNum == lastPktNum {
			// This is a message completion packet

			logger.Infof("Message transfer completed for %v", srcAddr)

			completeMsg, err := msgReconstructor.FinishMsgPacketSequence()
			if err != nil {
				logger.Warnf("Failed to finish packet sequence: %v", err)
			}

			reconstruction.ClearMsgReconstructor(srcAddr)

			fmt.Printf("MSG %v: %s\n", srcAddr, completeMsg)
			return
		}
	}

	logger.Warnf("Received FINISH packet of %v with last packet number %d, but no reconstructor found", srcAddr, lastPktNum)
}

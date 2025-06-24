package cmd

import (
	"fmt"
	"net/netip"
	"strings"
	"sync"

	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/util/logger"
)

func HandleSend(args []string) {
	if len(args) < 2 {
		println("Usage: msg <IPv4 address> <message>")
		return
	}

	peerIP, err := netip.ParseAddr(args[0])
	if err != nil || !peerIP.Is4() {
		println("Invalid IPv4 address:", args[0])
		return
	}

	blocker := sequencing.GetSequenceBlocker(peerIP, pkt.MsgTypeChatMessage)
	success := blocker.Block()
	if !success {
		fmt.Printf("Can't send message to %s: Another message is currently being sent.\n", peerIP)
		return
	}

	wg := &sync.WaitGroup{}

	fullMsg := strings.Join(args[1:], " ")
	msgBytes := []byte(fullMsg)
	bytesLen := len(msgBytes)

	var lastChunkPktNum [4]byte

	start := 0
	for start < bytesLen {
		end := min(start+common.MAX_PAYLOAD_SIZE_BYTES, bytesLen)

		packet := connection.BuildSequencedPacket(pkt.MsgTypeChatMessage, msgBytes[start:end], peerIP)

		lastChunkPktNum = packet.Header.PktNum

		wg.Add(1)
		go func() {
			defer wg.Done()
			<-outSequencing.SubscribeToReceivedAck(packet)
			// We ignore the success of the ACK to avoid blocking the send process. The receiver might get a faulty message.
		}()

		err := connection.SendReliableRoutedPacket(packet)
		if err != nil {
			logger.Warnf("Failed to send message to %s: %v\n", peerIP, err)
			// Don't return, send the remaining chunks anyway.
		}

		start = end
	}

	// Send the FIN message after all chunks have been sent and acknowledged
	go func() {
		wg.Wait() // TODO sometimes wait forever?

		payload := []byte(lastChunkPktNum[:])
		packet := connection.BuildSequencedPacket(pkt.MsgTypeFinish, payload, peerIP)

		go func() {
			<-outSequencing.SubscribeToReceivedAck(packet)
			// We ignore the success of the ACK to avoid blocking the send process. The receiver might not be ready for a new message but we don't care.
			blocker.Unblock()
		}()

		err := connection.SendReliableRoutedPacket(packet)
		if err != nil {
			logger.Warnf("Failed to send finish message to %s: %v\n", peerIP, err)
			return
		}
	}()
}

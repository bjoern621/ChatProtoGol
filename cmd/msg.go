package cmd

import (
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"time"

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

	go sendMsgChunks(peerIP, strings.Join(args[1:], " "), blocker)
}

func sendMsgChunks(peerIP netip.Addr, fullMsg string, blocker *sequencing.SequenceBlocker) {
	defer blocker.Unblock()

	wg := &sync.WaitGroup{}

	msgBytes := []byte(fullMsg)
	bytesLen := len(msgBytes)

	var lastChunkPktNum [4]byte

	start := 0
	for start < bytesLen {
		end := min(start+common.MAX_PAYLOAD_SIZE_BYTES, bytesLen)

		packet := connection.BuildSequencedPacket(pkt.MsgTypeChatMessage, msgBytes[start:end], peerIP)

		ackChan, err := connection.SendReliableRoutedPacket(packet)
		for err != nil {
			time.Sleep(common.SEQUENCE_RETRY_DELAY)
			logger.Debugf("Failed to send message chunk %v to %s, retrying: %v", packet.Header.PktNum, peerIP, err)
			ackChan, err = connection.SendReliableRoutedPacket(packet)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			<-ackChan
			// We ignore the success of the ACK to avoid blocking the send process. The receiver might get a faulty message.
		}()

		lastChunkPktNum = packet.Header.PktNum
		start = end
	}

	// Send the FIN message after all chunks have been sent and acknowledged
	wg.Wait()

	payload := []byte(lastChunkPktNum[:])
	packet := connection.BuildSequencedPacket(pkt.MsgTypeFinish, payload, peerIP)

	ackChan, err := connection.SendReliableRoutedPacket(packet)
	for err != nil {
		time.Sleep(common.SEQUENCE_RETRY_DELAY)
		logger.Debugf("Failed to send finish message to %s: %v\n", peerIP, err)
		ackChan, err = connection.SendReliableRoutedPacket(packet)
	}

	<-ackChan
	// We ignore the success of the ACK to avoid blocking the send process. The receiver might not be ready for a new message but we don't care.

	fmt.Printf("Message sent\n")
}

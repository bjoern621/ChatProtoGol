package cmd

import (
	"net/netip"
	"strings"

	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/util/logger"
)

func HandleSend(args []string) { // TODO blocking
	if len(args) < 2 {
		println("Usage: msg <IPv4 address> <message>")
		return
	}

	peerIP, err := netip.ParseAddr(args[0])
	if err != nil || !peerIP.Is4() {
		println("Invalid IPv4 address:", args[0])
		return
	}

	fullMsg := strings.Join(args[1:], " ")
	msgBytes := []byte(fullMsg)
	bytesLen := len(msgBytes)

	start := 0
	for start < bytesLen {
		end := min(start+common.MAX_PAYLOAD_SIZE_BYTES, bytesLen)

		payload := msgBytes[start:end]
		isLastPacket := end == bytesLen

		packet := connection.BuildSequencedPacket(pkt.MsgTypeChatMessage, isLastPacket, payload, peerIP)
		err := connection.SendReliableRoutedPacket(packet)
		if err != nil {
			logger.Warnf("Failed to send message to %s: %v\n", peerIP, err)
			return
		}

		start = end
	}
}

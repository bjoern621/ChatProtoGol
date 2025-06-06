// Package handler handles incoming packets from the UDP socket.
// It subscribes to the socket's packet observable and processes incoming packets.
package handler

import (
	"bjoernblessin.de/chatprotogol/protocol"
	"bjoernblessin.de/chatprotogol/socket"
	"bjoernblessin.de/chatprotogol/util/logger"
)

func ListenToPackets() {
	go func() {
		for packet := range socket.Subscribe() {
			processPacket(packet)
		}
	}()
}

func processPacket(packetBytes []byte) {
	packet, err := protocol.ParsePacket(packetBytes)
	if err != nil {
		logger.Warnf("Failed to parse packet: %v", err)
		return
	}

	logger.Infof("Parsed packet: %v", packet)
}

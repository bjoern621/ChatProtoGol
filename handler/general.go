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

	isValid := protocol.VerifyChecksum(packet)
	if !isValid {
		logger.Warnf("Invalid checksum for packet from %v to %v, received checksum: 0x%04X", packet.Header.SourceAddr, packet.Header.DestAddr, packet.Header.Checksum)
		return
	}

	switch packet.GetMessageType() {
	case protocol.MsgTypeConnect:
		handleConnect(packet)
	default:
		logger.Warnf("Unhandled packet type: %v from %v to %v", packet.GetMessageType(), packet.Header.SourceAddr, packet.Header.DestAddr)
		return
	}
}

func handleConnect(packet *protocol.Packet) {
	logger.Infof("Connection established with %v", packet.Header.SourceAddr)
}

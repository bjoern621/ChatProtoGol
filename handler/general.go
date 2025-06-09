// Package handler handles incoming packets from the UDP socket.
// It subscribes to the socket's packet observable and processes incoming packets.
package handler

import (
	"bjoernblessin.de/chatprotogol/pkt"
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

// processPacket processes an incoming UDP packet.
// It parses the packet, verifies the checksum, checks TTL and handles it based on its message type.
// This is the general entry for all incoming packets.
func processPacket(udpPacket *socket.Packet) {
	packet, err := pkt.ParsePacket(udpPacket.Data)
	if err != nil {
		logger.Warnf("Failed to parse packet: %v", err)
		return
	}

	isValid := pkt.VerifyChecksum(packet)
	if !isValid {
		logger.Warnf("Invalid checksum for packet from %v to %v, received checksum: 0x%04X", packet.Header.SourceAddr, packet.Header.DestAddr, packet.Header.Checksum)
		return
	}

	if packet.Header.TTL <= 0 {
		logger.Infof("Received message with TTL <= 0, dropping packet\n")
		return
	}

	switch packet.GetMessageType() {
	case pkt.MsgTypeConnect:
		handleConnect(packet, udpPacket.Addr)
	case pkt.MsgTypeDisconnect:
		handleDisconnect(packet, udpPacket.Addr)
	case pkt.MsgTypeRoutingTableUpdate:
		handleRoutingTableUpdate(packet, udpPacket.Addr)
	case pkt.MsgTypeAcknowledgment:
		handleAck(packet)
	case pkt.MsgTypeChatMessage:
		handleMsg(packet)
	default:
		logger.Warnf("Unhandled packet type: %v from %v to %v", packet.GetMessageType(), packet.Header.SourceAddr, packet.Header.DestAddr)
		return
	}
}

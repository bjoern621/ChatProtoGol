// Package handler handles incoming packets from the UDP socket.
// It subscribes to the socket's packet observable and processes incoming packets.
package handler

import (
	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/routing"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/sock"
	"bjoernblessin.de/chatprotogol/util/logger"
)

type PacketHandler struct {
	socket        sock.Socket
	router        *routing.Router
	inSequencing  *sequencing.IncomingPktNumHandler
	outSequencing *sequencing.OutgoingPktNumHandler
}

func NewPacketHandler(socket sock.Socket, router *routing.Router, inSequencing *sequencing.IncomingPktNumHandler, outSequencing *sequencing.OutgoingPktNumHandler) *PacketHandler {
	return &PacketHandler{
		socket:        socket,
		router:        router,
		inSequencing:  inSequencing,
		outSequencing: outSequencing,
	}
}

// ListenToPackets starts listening to incoming packets on the socket.
// It should be called in a separate goroutine to avoid blocking.
func (ph *PacketHandler) ListenToPackets() {
	var sem = make(chan struct{}, common.PACKET_HANDLER_GOROUTINES)

	for packet := range ph.socket.Subscribe() {
		select {
		case sem <- struct{}{}: // Acquire a semaphore slot
			go func() {
				ph.processPacket(packet)
				<-sem // Release the semaphore slot
			}()
		default:
			logger.Tracef("Packet handler is busy, dropping packet from %v", packet.Addr.AddrPort())
		}
	}
}

// processPacket processes an incoming UDP packet.
// It parses the packet, verifies the checksum, checks TTL and handles it based on its message type.
// This is the general entry for all incoming packets.
func (ph *PacketHandler) processPacket(udpPacket *sock.Packet) {
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
		logger.Warnf("Received message with TTL <= 0, dropping packet")
		return
	}

	logger.Tracef(packet.String())

	// TODO handle duplicates for packets that have destaddr == localaddress

	switch packet.GetMessageType() {
	case pkt.MsgTypeConnect:
		handleConnect(packet, udpPacket.Addr.AddrPort(), ph.router, ph.inSequencing, ph.socket)
	case pkt.MsgTypeDisconnect:
		handleDisconnect(packet, ph.inSequencing, ph.router, ph.socket, udpPacket.Addr.AddrPort())
	case pkt.MsgTypeAcknowledgment:
		handleAck(packet, ph.socket, ph.outSequencing)
	case pkt.MsgTypeChatMessage:
		handleMsg(packet, ph.socket, ph.inSequencing)
	case pkt.MsgTypeDD:
		handleDatabaseDescription(packet, ph.router, ph.inSequencing, udpPacket.Addr.AddrPort(), ph.socket)
	case pkt.MsgTypeLSA:
		handleLSA(packet, ph.router, ph.inSequencing, udpPacket.Addr.AddrPort(), ph.socket)
	case pkt.MsgTypeFinish:
		handleFinish(packet, ph.inSequencing, ph.socket)
	case pkt.MsgTypeFileTransfer:
		handleFileTransfer(packet, ph.socket, ph.inSequencing)
	default:
		logger.Warnf("Unhandled packet type: %v from %v to %v", packet.GetMessageType(), packet.Header.SourceAddr, packet.Header.DestAddr)
		return
	}
}

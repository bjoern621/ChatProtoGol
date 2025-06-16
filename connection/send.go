// Pacage connection provides high-level functions for sending to peers.
package connection

import (
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"slices"

	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/reconstruction"
	"bjoernblessin.de/chatprotogol/routing"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/sock"
	"bjoernblessin.de/chatprotogol/util/logger"
)

var socket sock.Socket
var router *routing.Router
var incomingSequencing *sequencing.IncomingPktNumHandler
var outgoingSequencing *sequencing.OutgoingPktNumHandler
var reconstructor *reconstruction.PktSequenceReconstructor

func SetGlobalVars(s sock.Socket, r *routing.Router, in *sequencing.IncomingPktNumHandler, out *sequencing.OutgoingPktNumHandler, recon *reconstruction.PktSequenceReconstructor) {
	socket = s
	router = r
	incomingSequencing = in
	outgoingSequencing = out
	reconstructor = recon
}

var msgTypeNames = map[byte]string{
	pkt.MsgTypeConnect:        "CONN",
	pkt.MsgTypeDisconnect:     "DIS",
	pkt.MsgTypeChatMessage:    "MSG",
	pkt.MsgTypeFileTransfer:   "FILE",
	pkt.MsgTypeAcknowledgment: "ACK",
	pkt.MsgTypeLSA:            "LSA",
	pkt.MsgTypeDD:             "DD",
}

// SendReliableRoutedPacket sends a packet.
// Reliable: Resends and timeouts are handled.
// Routed: Uses the routing table to determine the next hop.
func SendReliableRoutedPacket(packet *pkt.Packet) error {
	destinationIP := netip.AddrFrom4(packet.Header.DestAddr)

	nextHop, found := router.GetNextHop(destinationIP)
	if !found {
		return errors.New("no next hop found for the destination address")
	}

	err := sendPacketTo(nextHop, packet)
	if err != nil {
		return err
	}

	outgoingSequencing.AddOpenAck(destinationIP, packet.Header.PktNum, func() {
		nextHop, found := router.GetNextHop(destinationIP) // Get the current next hop again (it may have changed)
		if !found {
			logger.Infof("Host %s is no longer reachable, removing open acknowledgment for packet number %v", destinationIP, packet.Header.PktNum)
			return // Peer no longer reachable (e.g., disconnected)
		}

		_ = sendPacketTo(nextHop, packet)
	})

	return nil
}

// SendReliablePacketTo sends a packet.
// Reliable: Resends and timeouts are handled.
// To: Send the packet to a specific address and port.
func SendReliablePacketTo(addrPort netip.AddrPort, packet *pkt.Packet) error {
	destinationAddr := addrPort.Addr()

	err := sendPacketTo(addrPort, packet)
	if err != nil {
		return err
	}

	outgoingSequencing.AddOpenAck(destinationAddr, packet.Header.PktNum, func() {
		_ = sendPacketTo(addrPort, packet)
	})

	return nil
}

// sendPacketTo sends a packet to an AddrPort.
func sendPacketTo(addrPort netip.AddrPort, packet *pkt.Packet) error {
	nextHop := &net.UDPAddr{
		IP:   addrPort.Addr().AsSlice(),
		Port: int(addrPort.Port()),
	}

	err := socket.SendTo(nextHop, packet.ToByteArray())
	if err != nil {
		return errors.New("failed to send packet to peer: " + err.Error())
	}

	logger.Infof("SENT %s %d to %v", msgTypeNames[packet.GetMessageType()], packet.Header.PktNum, packet.Header.DestAddr)

	return nil
}

// BuildSequencedPacket constructs a packet with the next packet number for the destination address.
func BuildSequencedPacket(msgType byte, lastBit bool, payload pkt.Payload, destAddr netip.Addr) *pkt.Packet {
	return buildPacket(msgType, lastBit, payload, destAddr, outgoingSequencing.GetNextpacketNumber(destAddr))
}

func buildPacket(msgType byte, lastBit bool, payload pkt.Payload, destAddr netip.Addr, pktNum [4]byte) *pkt.Packet {
	packet := &pkt.Packet{
		Header: pkt.Header{
			SourceAddr: socket.MustGetLocalAddress().Addr().As4(),
			DestAddr:   destAddr.As4(),
			Control:    pkt.MakeControlByte(msgType, lastBit, common.TEAM_ID),
			TTL:        common.INITIAL_TTL,
			PktNum:     pktNum,
		},
		Payload: payload,
	}
	pkt.SetChecksum(packet)
	return packet
}

// SendRoutedAcknowledgment sends an acknowledgment packet to the specified peer address.
// Routed: Uses the routing table to determine the next hop.
func SendRoutedAcknowledgment(peerAddr netip.Addr, pktNum [4]byte) error {
	ackPacket := buildPacket(pkt.MsgTypeAcknowledgment, true, nil, peerAddr, pktNum)

	nextHop, found := router.GetNextHop(peerAddr)
	if !found {
		return errors.New("no next hop found for the peer address (is the peer disconnected?)")
	}

	err := sendPacketTo(nextHop, ackPacket)
	if err != nil {
		return err
	}

	return nil
}

// FloodLSA sends a Link State Advertisement (LSA) to all neighbors.
// Optionally, it can exclude certain addresses (neighbors) from receiving the LSA.
func FloodLSA(lsaOwner netip.Addr, lsa routing.LSAEntry, exceptAddrs ...netip.Addr) {
	payload := make(pkt.Payload, 0, 8+len(lsa.Neighbors)*4)

	lsaOwnerBytes := lsaOwner.As4()
	payload = append(payload, lsaOwnerBytes[:]...)

	seqNumBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(seqNumBytes, lsa.SeqNum)
	payload = append(payload, seqNumBytes...)

	for _, neighborAddr := range lsa.Neighbors {
		addrBytes := neighborAddr.As4()
		payload = append(payload, addrBytes[:]...)
	}

	for destinationAddr := range router.GetNeighbors() {
		if slices.Contains(exceptAddrs, destinationAddr) {
			continue
		}

		packet := BuildSequencedPacket(pkt.MsgTypeLSA, true, payload, destinationAddr)

		err := SendReliableRoutedPacket(packet)
		if err != nil {
			logger.Warnf("Failed to send LSA for %s: %v", destinationAddr, err)
		}
	}
}

// SendDD sends a Database Description representing our LSDB to the destination address.
func SendDD(destAddr netip.Addr) error {
	existingLSAs := router.GetAvailableLSAs()
	payload := make(pkt.Payload, 0, len(existingLSAs))
	for _, addr := range existingLSAs {
		addrBytes := addr.As4()
		payload = append(payload, addrBytes[:]...)
	}

	packet := BuildSequencedPacket(pkt.MsgTypeDD, true, payload, destAddr)

	return SendReliableRoutedPacket(packet)
}

// ForwardRouted forwards a packet to the destination address defined in the packet header.
// Routed: Uses the routing table to determine the next hop.
// This function automatically decrements the TTL by one.
// Timeouts and resends are NOT handled (should be handled by source peer).
// Errors if the TTL is already zero or less.
func ForwardRouted(packet *pkt.Packet) error {
	destinationIP := netip.AddrFrom4(packet.Header.DestAddr)

	nextHop, found := router.GetNextHop(destinationIP)
	if !found {
		return errors.New("no next hop found for the destination address")
	}

	err := sendPacketTo(nextHop, packet)
	if err != nil {
		return err
	}

	logger.Infof("FORWARDED %s %d to %v", msgTypeNames[packet.GetMessageType()], packet.Header.PktNum, packet.Header.DestAddr)

	return nil
}

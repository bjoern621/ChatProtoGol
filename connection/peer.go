// Package connection manages the connection to a peer.
// It handles the routing table, sequence numbers, and connection management.
package connection

import (
	"errors"
	"net"
	"net/netip"

	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/reconstruction"
	"bjoernblessin.de/chatprotogol/routing"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/sock"
	"bjoernblessin.de/chatprotogol/util/assert"
	"bjoernblessin.de/chatprotogol/util/logger"
)

var socket sock.Socket
var router *routing.Router
var incomingSequencing *sequencing.IncomingPktNumHandler
var outgoingSequencing *sequencing.OutgoingPktNumHandler

func SetGlobalVars(s sock.Socket, r *routing.Router, in *sequencing.IncomingPktNumHandler, out *sequencing.OutgoingPktNumHandler) {
	socket = s
	router = r
	incomingSequencing = in
	outgoingSequencing = out
}

// Peer represents another host in the network.
type Peer struct {
	Address netip.Addr
}

// GetPeer retrieves a peer by its address.
// It always returns a new Peer instance.
// Uses the routing table to check if the peer exists.
func GetPeer(addr netip.Addr) (p *Peer, exists bool) {
	_, exists = routing.GetRoutingTableEntries()[addr]
	if !exists {
		return nil, false
	}
	return &Peer{Address: addr}, true
}

// Delete removes the peer from the managed peers.
// This should be called when the peer is no longer needed, such as after a disconnect.
// Also clears routing entries, sequence numbers, payload buffers (msg / file transfer).
func (p *Peer) Delete() {
	if is, addrPort := routing.IsNeighbor2(p.Address); is {
		// Remove all peers that are routed through this peer from the routing table.
		routing.RemoveRoutingEntriesWithNextHop(addrPort)
	} else {
		// If the peer is not a neighbor, we still need to clear the peer's routing entry.
		routing.RemoveRoutingEntry(p.Address)
	}
	outgoingSequencing.ClearSequenceNumbers(p.Address)
	incomingSequencing.ClearIncomingSequenceNumbers(p.Address)
	reconstruction.ClearPayloadBuffer(p.Address)
}

// SendNewTo sends a packet to the peer at the specified address and port.
// Timeouts and resends are handled.
// It returns the sequence number used for this packet.
func SendNewTo(addrPort netip.AddrPort, msgType byte, lastBit bool, payload pkt.Payload, destinationIP netip.Addr, seqNum [4]byte) error {
	packet, err := sendNewTo(addrPort, msgType, lastBit, payload, seqNum, destinationIP)
	if err != nil {
		return err
	}

	outgoingSequencing.AddOpenAck(destinationIP, packet.Header.PktNum, func() {
		nextHop, found := routing.GetNextHop2(destinationIP)
		if !found {
			logger.Infof("Peer %s is no longer reachable, removing open acknowledgment for sequence number %v", destinationIP, packet.Header.PktNum)
			return // Peer no longer reachable (e.g., disconnected)
		}

		_ = SendPacketTo(nextHop, packet)
	})

	return nil
}

// sendNewTo is a helper function that constructs and sends a packet to the peer.
// It does not handle timeouts or resends.
func sendNewTo(addrPort netip.AddrPort, msgType byte, lastBit bool, payload pkt.Payload, seqNum [4]byte, destinationIP netip.Addr) (*pkt.Packet, error) {
	packet := &pkt.Packet{
		Header: pkt.Header{
			SourceAddr: socket.MustGetLocalAddress().Addr().As4(),
			DestAddr:   destinationIP.As4(),
			Control:    pkt.MakeControlByte(msgType, lastBit, common.TEAM_ID),
			TTL:        common.INITIAL_TTL,
			PktNum:     seqNum,
		},
		Payload: payload,
	}
	pkt.SetChecksum(packet)

	err := SendPacketTo(addrPort, packet)
	if err != nil {
		return nil, err
	}

	return packet, nil
}

var msgTypeNames = map[byte]string{
	pkt.MsgTypeConnect:            "CONN",
	pkt.MsgTypeDisconnect:         "DIS",
	pkt.MsgTypeRoutingTableUpdate: "ROUTING",
	pkt.MsgTypeChatMessage:        "MSG",
	pkt.MsgTypeFileTransfer:       "FILE",
	pkt.MsgTypeResendRequest:      "RESEND",
	pkt.MsgTypeAcknowledgment:     "ACK",
	pkt.MsgTypeLSA:                "LSA",
	pkt.MsgTypeDD:                 "DD",
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

	err := SendPacketTo(nextHop, packet)
	if err != nil {
		return err
	}

	outgoingSequencing.AddOpenAck(destinationIP, packet.Header.PktNum, func() {
		nextHop, found := router.GetNextHop(destinationIP) // Get the current next hop again (it may have changed)
		if !found {
			logger.Infof("Peer %s is no longer reachable, removing open acknowledgment for packet number %v", destinationIP, packet.Header.PktNum)
			return // Peer no longer reachable (e.g., disconnected)
		}

		_ = SendPacketTo(nextHop, packet)
	})

	return nil
}

// SendReliablePacketTo sends a packet.
// Reliable: Resends and timeouts are handled.
// To: Send the packet to a specific address and port.
func SendReliablePacketTo(addrPort netip.AddrPort, packet *pkt.Packet) error {
	destinationAddr := addrPort.Addr()

	err := SendPacketTo(addrPort, packet)
	if err != nil {
		return err
	}

	outgoingSequencing.AddOpenAck(destinationAddr, packet.Header.PktNum, func() {
		_ = SendPacketTo(addrPort, packet)
	})

	return nil
}

// SendPacketTo sends a packet.
func SendPacketTo(addrPort netip.AddrPort, packet *pkt.Packet) error {
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

// SendNew sends a packet to the peer using the routing table.
// Timeouts and resends are handled.
func (p *Peer) SendNew(msgType byte, lastBit bool, payload pkt.Payload) error {
	nextHopAddrPort, found := routing.GetNextHop2(p.Address)
	if !found {
		return errors.New("no next hop found for the peer")
	}

	seqNum := outgoingSequencing.GetNextSequenceNumber(p.Address)

	return SendNewTo(nextHopAddrPort, msgType, lastBit, payload, p.Address, seqNum)
}

// Forward forwards a packet to the peer.
// This function automatically decrements the TTL by one.
// Timeouts and resends are NOT handled (should be handled by source peer).
// Errors if the TTL is already zero or less.
func (p *Peer) Forward(packet *pkt.Packet) error {
	if packet.Header.TTL <= 0 {
		return errors.New("cannot forward packet with TTL <= 0")
	}
	packet.Header.TTL--

	pkt.SetChecksum(packet)

	destPeer, found := GetPeer(netip.AddrFrom4(packet.Header.DestAddr))
	if !found {
		return errors.New("no peer found for destination address")
	}

	nextHop, exists := routing.GetNextHop2(destPeer.Address)
	assert.Assert(exists == true, "Next hop should not be nil because a peer was found")

	err := SendPacketTo(nextHop, packet)
	if err != nil {
		return err
	}

	return nil
}

// SendNewAll sends a packet to all peers in the provided peer map.
// Timeouts and resends are handled.
func SendNewAll(msgType byte, lastBit bool, payload pkt.Payload, peerMap map[netip.Addr]*Peer) error {
	for _, peer := range peerMap {
		err := peer.SendNew(msgType, lastBit, payload)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Peer) SendAcknowledgment(seqNum [4]byte) {
	ackPacket := &pkt.Packet{
		Header: pkt.Header{
			SourceAddr: socket.MustGetLocalAddress().Addr().As4(),
			DestAddr:   p.Address.As4(),
			Control:    pkt.MakeControlByte(pkt.MsgTypeAcknowledgment, true, common.TEAM_ID),
			TTL:        common.INITIAL_TTL,
			PktNum:     seqNum,
		},
	}
	pkt.SetChecksum(ackPacket)

	nextHop, found := routing.GetNextHop2(p.Address)
	assert.Assert(found, "Next hop should not be nil when sending acknowledgment")

	err := SendPacketTo(nextHop, ackPacket)
	if err != nil {
		logger.Warnf("Failed to send acknowledgment to peer %v: %v", p.Address, err)
		return
	}

	return
}

// GetAllNeighbors returns a map of all neighbors (peers that are directly connected).
func GetAllNeighbors() map[netip.Addr]*Peer {
	neighbors := make(map[netip.Addr]*Peer)

	for addr := range routing.GetRoutingTableEntries() {
		if is, _ := routing.IsNeighbor2(addr); is {
			neighbors[addr] = &Peer{Address: addr}
		}
	}

	return neighbors
}

// SendCurrentRoutingTable sends the current routing table to all specified peers.
func SendCurrentRoutingTable(peerMap map[netip.Addr]*Peer) {
	payload := routing.FormatRoutingTableForPayload()

	err := SendNewAll(pkt.MsgTypeRoutingTableUpdate, true, payload, peerMap)
	if err != nil {
		logger.Warnf("Failed to send routing table update at least one peer: %v", err)
		return
	}
}

// BuildSequencedPacket constructs a packet with the next packet number for the destination address.
func BuildSequencedPacket(msgType byte, lastBit bool, payload pkt.Payload, destAddr netip.Addr) *pkt.Packet {
	return buildPacket(msgType, lastBit, payload, destAddr, outgoingSequencing.GetNextSequenceNumber(destAddr))
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

func SendAcknowledgment(peerAddr netip.Addr, pktNum [4]byte) {
	ackPacket := buildPacket(pkt.MsgTypeAcknowledgment, true, nil, peerAddr, pktNum)

	nextHop, found := router.GetNextHop(peerAddr)
	assert.Assert(found, "Next hop should not be nil when sending acknowledgment")

	err := SendPacketTo(nextHop, ackPacket)
	if err != nil {
		logger.Warnf("Failed to send acknowledgment to peer %v: %v", peerAddr, err)
		return
	}

	return
}

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
	"bjoernblessin.de/chatprotogol/socket"
	"bjoernblessin.de/chatprotogol/util/assert"
	"bjoernblessin.de/chatprotogol/util/logger"
)

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
	if is, addrPort := routing.IsNeighbor(p.Address); is {
		// Remove all peers that are routed through this peer from the routing table.
		routing.RemoveRoutingEntriesWithNextHop(addrPort)
	} else {
		// If the peer is not a neighbor, we still need to clear the peer's routing entry.
		routing.RemoveRoutingEntry(p.Address)
	}
	sequencing.ClearSequenceNumbers(p.Address)
	sequencing.ClearIncomingSequenceNumbers(p.Address)
	reconstruction.ClearPayloadBuffer(p.Address)
}

// SendNewTo sends a packet to the peer at the specified address and port.
// Timeouts and resends are handled.
func SendNewTo(addrPort netip.AddrPort, msgType byte, lastBit bool, payload pkt.Payload, destinationIP netip.Addr) error {
	seqNum := sequencing.GetNextSequenceNumber(destinationIP)

	packet, err := sendNewTo(addrPort, msgType, lastBit, payload, seqNum, destinationIP)
	if err != nil {
		return err
	}

	sequencing.AddOpenAck(destinationIP, packet.Header.PktNum, func() {
		nextHop, found := routing.GetNextHop(destinationIP)
		if !found {
			logger.Infof("Peer %s is no longer reachable, removing open acknowledgment for sequence number %v", destinationIP, packet.Header.PktNum)
			return // Peer no longer reachable (e.g., disconnected)
		}

		_ = sendPacketTo(nextHop, packet)
	})

	return nil
}

// sendNewTo is a helper function that constructs and sends a packet to the peer.
// It does not handle timeouts or resends.
func sendNewTo(addrPort netip.AddrPort, msgType byte, lastBit bool, payload pkt.Payload, seqNum [4]byte, destinationIP netip.Addr) (*pkt.Packet, error) {
	packet := &pkt.Packet{
		Header: pkt.Header{
			SourceAddr: socket.GetLocalAddress().AddrPort().Addr().As4(),
			DestAddr:   destinationIP.As4(),
			Control:    pkt.MakeControlByte(msgType, lastBit, common.TEAM_ID),
			TTL:        common.INITIAL_TTL,
			PktNum:     seqNum,
		},
		Payload: payload,
	}
	pkt.SetChecksum(packet)

	err := sendPacketTo(addrPort, packet)
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
}

// sendPacketTo sends the packet to the specified address and port.
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

// SendNew sends a packet to the peer using the routing table.
// Timeouts and resends are handled.
func (p *Peer) SendNew(msgType byte, lastBit bool, payload pkt.Payload) error {
	nextHopAddrPort, found := routing.GetNextHop(p.Address)
	if !found {
		return errors.New("no next hop found for the peer")
	}

	return SendNewTo(nextHopAddrPort, msgType, lastBit, payload, p.Address)
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

	nextHop, exists := routing.GetNextHop(destPeer.Address)
	assert.Assert(exists == true, "Next hop should not be nil because a peer was found")

	err := sendPacketTo(nextHop, packet)
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
			SourceAddr: socket.GetLocalAddress().AddrPort().Addr().As4(),
			DestAddr:   p.Address.As4(),
			Control:    pkt.MakeControlByte(pkt.MsgTypeAcknowledgment, true, common.TEAM_ID),
			TTL:        common.INITIAL_TTL,
			PktNum:     seqNum,
		},
	}
	pkt.SetChecksum(ackPacket)

	nextHop, found := routing.GetNextHop(p.Address)
	assert.Assert(found, "Next hop should not be nil when sending acknowledgment")

	err := sendPacketTo(nextHop, ackPacket)
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
		if is, _ := routing.IsNeighbor(addr); is {
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

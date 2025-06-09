// Package connection manages the connection to a peer.
// It handles the routing table, sequence numbers, and connection management.
package connection

import (
	"errors"
	"net"
	"net/netip"

	"maps"

	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/socket"
	"bjoernblessin.de/chatprotogol/util/assert"
	"bjoernblessin.de/chatprotogol/util/logger"
)

type Peer struct {
	Address netip.Addr
}

var (
	peers = make(map[netip.Addr]*Peer) // Maps IPv4 addresses to Peer instances
)

// NewPeer creates a new Peer instance with the given address.
func NewPeer(address netip.Addr) *Peer {
	peer := &Peer{
		Address: address,
	}
	peers[address] = peer
	return peer
}

func GetPeer(addr netip.Addr) (p *Peer, exists bool) {
	peer, exists := peers[addr]
	if !exists {
		return nil, false
	}
	return peer, true
}

// Delete removes the peer from the managed peers.
// This should be called when the peer is no longer needed, such as after a disconnect.
func (p *Peer) Delete() {
	delete(peers, p.Address)
}

// SendNewTo sends a packet to the peer at the specified address and port.
// Timeouts and resends are handled.
func (p *Peer) SendNewTo(addrPort netip.AddrPort, msgType byte, lastBit bool, payload pkt.Payload) error {
	seqNum := getNextSequenceNumber(p)

	packet, err := p.sendNewTo(addrPort, msgType, lastBit, payload, seqNum)
	if err != nil {
		return err
	}

	addOpenAck(p, packet)

	return nil
}

// sendNewTo is a helper function that constructs and sends a packet to the peer.
// It does not handle timeouts or resends.
func (p *Peer) sendNewTo(addrPort netip.AddrPort, msgType byte, lastBit bool, payload pkt.Payload, seqNum [4]byte) (*pkt.Packet, error) {
	packet := &pkt.Packet{
		Header: pkt.Header{
			SourceAddr: socket.GetLocalAddress().AddrPort().Addr().As4(),
			DestAddr:   p.Address.As4(),
			Control:    pkt.MakeControlByte(msgType, lastBit, common.TEAM_ID),
			TTL:        common.INITIAL_TTL,
			SeqNum:     seqNum,
		},
		Payload: payload,
	}
	pkt.SetChecksum(packet)

	err := p.sendPacketTo(addrPort, packet)
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
func (p *Peer) sendPacketTo(addrPort netip.AddrPort, packet *pkt.Packet) error {
	nextHop := &net.UDPAddr{
		IP:   addrPort.Addr().AsSlice(),
		Port: int(addrPort.Port()),
	}

	err := socket.SendTo(nextHop, packet.ToByteArray())
	if err != nil {
		return errors.New("failed to send packet to peer: " + err.Error())
	}

	logger.Infof("SENT %s %d to %v", msgTypeNames[packet.GetMessageType()], packet.Header.SeqNum, packet.Header.DestAddr)

	return nil
}

// SendNew sends a packet to the peer using the routing table.
// Timeouts and resends are handled.
func (p *Peer) SendNew(msgType byte, lastBit bool, payload pkt.Payload) error {
	nextHopAddrPort, found := GetNextHop(p.Address)
	if !found {
		return errors.New("no next hop found for the peer")
	}

	return p.SendNewTo(nextHopAddrPort, msgType, lastBit, payload)
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

	nextHop, exists := GetNextHop(destPeer.Address)
	assert.Assert(exists == true, "Next hop should not be nil because a peer was found")

	p.sendPacketTo(nextHop, packet)

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
			SeqNum:     seqNum,
		},
	}
	pkt.SetChecksum(ackPacket)

	nextHop, found := GetNextHop(p.Address)
	assert.Assert(found, "Next hop should not be nil when sending acknowledgment")

	err := p.sendPacketTo(nextHop, ackPacket)
	if err != nil {
		logger.Warnf("Failed to send acknowledgment to peer %v: %v", p.Address, err)
		return
	}

	return
}

// GetAllPeers returns a copy of the current peers map.
func GetAllPeers() map[netip.Addr]*Peer {
	peersCopy := make(map[netip.Addr]*Peer, len(peers))

	maps.Copy(peersCopy, peers)
	return peersCopy
}

// GetAllNeighbors returns a map of all neighbors (peers that are directly connected).
func GetAllNeighbors() map[netip.Addr]*Peer {
	neighbors := make(map[netip.Addr]*Peer)

	for addr, peer := range peers {
		if IsNeighbor(addr) {
			neighbors[addr] = peer
		}
	}

	return neighbors
}

// SendCurrentRoutingTable sends the current routing table to all specified peers.
func SendCurrentRoutingTable(peerMap map[netip.Addr]*Peer) {
	payload := FormatRoutingTableForPayload()

	err := SendNewAll(pkt.MsgTypeRoutingTableUpdate, true, payload, peerMap)
	if err != nil {
		logger.Warnf("Failed to send routing table update at least one peer: %v", err)
		return
	}
}

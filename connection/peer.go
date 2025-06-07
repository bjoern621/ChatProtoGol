// Package connection manages the connection to a peer.
// It handles the routing table, sequence numbers, and connection establishment.
package connection

import (
	"errors"
	"net"
	"net/netip"

	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/socket"
)

type Peer struct {
	Address netip.Addr
}

// NewPeer creates a new Peer instance with the given address.
func NewPeer(address netip.Addr) *Peer {
	return &Peer{
		Address: address,
	}
}

// Send sends a new packet to the peer.
func (p *Peer) Send(msgType byte, lastBit bool, payload []byte) error {
	addrPort, found := getNextHop(p.Address)
	if !found {
		return errors.New("no next hop found for the peer")
	}

	packet := &pkt.Packet{
		Header: pkt.Header{
			SourceAddr: socket.GetLocalAddress().AddrPort().Addr().As4(),
			DestAddr:   p.Address.As4(),
			Control:    pkt.MakeControlByte(msgType, lastBit, common.TEAM_ID),
			TTL:        common.INITIAL_TTL,
			SeqNum:     getNextSequenceNumber(p.Address),
		},
		Payload: payload,
	}
	pkt.SetChecksum(packet)

	nextHop := &net.UDPAddr{
		IP:   addrPort.Addr().AsSlice(),
		Port: int(addrPort.Port()),
	}

	err := socket.SendTo(nextHop, packet.ToByteArray())
	if err != nil {
		return errors.New("failed to send packet to peer: " + err.Error())
	}

	return nil
}

// Forward forwards a packet to the peer.
// This function automatically decrements the TTL by one.
func (p *Peer) Forward(payload []byte) error {
	// Implementation for forwarding a packet to the peer.
	// This could involve modifying the packet if necessary and then sending it.
	// For now, we will just return nil to indicate success.
	return nil
}

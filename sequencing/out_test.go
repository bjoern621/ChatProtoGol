package sequencing

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"bjoernblessin.de/chatprotogol/pkt"
)

func makePkt(num uint32, dest netip.Addr) *pkt.Packet {
	var pktNum [4]byte
	binary.BigEndian.PutUint32(pktNum[:], num)
	return &pkt.Packet{
		Header: pkt.Header{
			DestAddr: dest.As4(),
			PktNum:   pktNum,
		},
	}
}

func TestSenderWindowBlocks(t *testing.T) {
	window := int64(3)

	out := NewOutgoingPktNumHandler()
	dest, _ := netip.ParseAddr("10.0.0.1")

	// Cannot send too far ahead packet
	pktTooFar := makePkt(uint32(window+10), dest)
	_, err := out.AddOpenAck(pktTooFar, func() {})
	if err == nil {
		t.Fatalf("expected error when sending packet too far ahead, got nil")
	}

	// Fill the window
	for i := range window {
		pkt := makePkt(uint32(i), dest)
		_, err := out.AddOpenAck(pkt, func() {})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Next send should fail (window full)
	// pkt := makePkt(uint32(window), dest)
	// _, err = out.AddOpenAck(pkt, func() {})
	// if err == nil {
	// 	t.Fatalf("expected error when window is full, got nil")
	// }

	// Still cannot send too far ahead packet
	_, err = out.AddOpenAck(pktTooFar, func() {})
	if err == nil {
		t.Fatalf("expected error when sending packet too far ahead, got nil")
	}

	// Remove one ack, should allow another send
	out.RemoveOpenAck(dest, makePkt(0, dest).Header.PktNum)
	_, err = out.AddOpenAck(makePkt(uint32(window), dest), func() {})
	if err != nil {
		t.Fatalf("expected to send after ack, got error: %v", err)
	}

	// Still cannot send too far ahead packet
	_, err = out.AddOpenAck(pktTooFar, func() {})
	if err == nil {
		t.Fatalf("expected error when sending packet too far ahead, got nil")
	}
}

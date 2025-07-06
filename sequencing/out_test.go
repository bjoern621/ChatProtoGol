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

	out := NewOutgoingPktNumHandler(window)
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
	pkt := makePkt(uint32(window), dest)
	_, err = out.AddOpenAck(pkt, func() {})
	if err == nil {
		t.Fatalf("expected error when window is full, got nil")
	}

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

func TestHighestAckedAdvancementWhenAllPacketsAcked(t *testing.T) {
	handler := NewOutgoingPktNumHandler(10)
	addr := netip.MustParseAddr("192.168.1.1")

	// Send packets 0, 1, 2, 3
	var packets []*pkt.Packet

	for i := 0; i < 4; i++ {
		packet := makePkt(uint32(i), addr)
		packets = append(packets, packet)

		// Manually update the packet counter to match what GetNextpacketNumber would do
		handler.packetNumbers[addr] = uint32(i + 1)

		_, err := handler.AddOpenAck(packet, func() {})
		if err != nil {
			t.Fatalf("Failed to add open ack for packet %d: %v", i, err)
		}
	}

	// Verify initial state: highest should be -1 (no packets ACKed yet)
	if handler.highestAckedContiguousPktNum[addr] != -1 {
		t.Errorf("Expected highest acked to be -1, got %d", handler.highestAckedContiguousPktNum[addr])
	}

	// ACK packets in order: 0, 1, 2
	for i := 0; i < 3; i++ {
		handler.RemoveOpenAck(addr, packets[i].Header.PktNum)

		// After each ACK, highest should advance
		expected := int64(i)
		if handler.highestAckedContiguousPktNum[addr] != expected {
			t.Errorf("After ACKing packet %d, expected highest acked to be %d, got %d",
				i, expected, handler.highestAckedContiguousPktNum[addr])
		}
	}

	// ACK the final packet
	handler.RemoveOpenAck(addr, packets[3].Header.PktNum)

	// After ACKing packet 3, highest should advance to 3
	expected := int64(3)
	if handler.highestAckedContiguousPktNum[addr] != expected {
		t.Errorf("After ACKing final packet 3, expected highest acked to be %d, got %d",
			expected, handler.highestAckedContiguousPktNum[addr])
	}

	// Verify that openAcks for this addr has been deleted (expected behavior)
	if _, exists := handler.openAcks[addr]; exists {
		t.Error("Expected openAcks[addr] to be deleted after all packets ACKed")
	}
}

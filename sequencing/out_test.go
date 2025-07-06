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

func TestCongestionAvoidanceAccumulatorReset(t *testing.T) {
	handler := NewOutgoingPktNumHandler(2) // Small initial cwnd for faster testing
	addr := netip.MustParseAddr("192.168.1.1")

	// Force into congestion avoidance phase by setting ssthresh low
	handler.ssthresh[addr] = 1
	handler.cwnd[addr] = 2 // Start with cwnd = 2
	handler.cAvoidanceAcc[addr] = 0

	// Initial state: cwnd=2, accumulator=0
	if handler.cwnd[addr] != 2 {
		t.Errorf("Expected initial cwnd to be 2, got %d", handler.cwnd[addr])
	}
	if handler.cAvoidanceAcc[addr] != 0 {
		t.Errorf("Expected initial accumulator to be 0, got %d", handler.cAvoidanceAcc[addr])
	}

	// Send packet 0
	packet0 := makePkt(uint32(0), addr)
	handler.packetNumbers[addr] = 1
	_, err := handler.AddOpenAck(packet0, func() {})
	if err != nil {
		t.Fatalf("Failed to add open ack for packet 0: %v", err)
	}

	// Send packet 1
	packet1 := makePkt(uint32(1), addr)
	handler.packetNumbers[addr] = 2
	_, err = handler.AddOpenAck(packet1, func() {})
	if err != nil {
		t.Fatalf("Failed to add open ack for packet 1: %v", err)
	}

	// ACK packet 0: accumulator should become 1, cwnd stays 2
	handler.RemoveOpenAck(addr, packet0.Header.PktNum)
	if handler.cwnd[addr] != 2 {
		t.Errorf("After 1st ACK, expected cwnd to be 2, got %d", handler.cwnd[addr])
	}
	if handler.cAvoidanceAcc[addr] != 1 {
		t.Errorf("After 1st ACK, expected accumulator to be 1, got %d", handler.cAvoidanceAcc[addr])
	}

	// Now we can send packet 2 (window has room)
	packet2 := makePkt(uint32(2), addr)
	handler.packetNumbers[addr] = 3
	_, err = handler.AddOpenAck(packet2, func() {})
	if err != nil {
		t.Fatalf("Failed to add open ack for packet 2: %v", err)
	}

	// ACK packet 1: accumulator reaches cwnd (2), should trigger window increase and reset
	handler.RemoveOpenAck(addr, packet1.Header.PktNum)
	if handler.cwnd[addr] != 3 {
		t.Errorf("After 2nd ACK, expected cwnd to be 3, got %d", handler.cwnd[addr])
	}
	if handler.cAvoidanceAcc[addr] != 0 {
		t.Errorf("After 2nd ACK, expected accumulator to be reset to 0, got %d", handler.cAvoidanceAcc[addr])
	}

	// Now we can send packet 3 (window increased to 3)
	packet3 := makePkt(uint32(3), addr)
	handler.packetNumbers[addr] = 4
	_, err = handler.AddOpenAck(packet3, func() {})
	if err != nil {
		t.Fatalf("Failed to add open ack for packet 3: %v", err)
	}

	// ACK packet 2: accumulator should become 1 again
	handler.RemoveOpenAck(addr, packet2.Header.PktNum)
	if handler.cwnd[addr] != 3 {
		t.Errorf("After 3rd ACK, expected cwnd to stay 3, got %d", handler.cwnd[addr])
	}
	if handler.cAvoidanceAcc[addr] != 1 {
		t.Errorf("After 3rd ACK, expected accumulator to be 1, got %d", handler.cAvoidanceAcc[addr])
	}

	// ACK packet 3: accumulator becomes 2
	handler.RemoveOpenAck(addr, packet3.Header.PktNum)
	if handler.cwnd[addr] != 3 {
		t.Errorf("After 4th ACK, expected cwnd to stay 3, got %d", handler.cwnd[addr])
	}
	if handler.cAvoidanceAcc[addr] != 2 {
		t.Errorf("After 4th ACK, expected accumulator to be 2, got %d", handler.cAvoidanceAcc[addr])
	}

	// Send and ACK one more packet to trigger the next window increase
	packet4 := makePkt(uint32(4), addr)
	handler.packetNumbers[addr] = 5
	_, err = handler.AddOpenAck(packet4, func() {})
	if err != nil {
		t.Fatalf("Failed to add open ack for packet 4: %v", err)
	}

	handler.RemoveOpenAck(addr, packet4.Header.PktNum)
	if handler.cwnd[addr] != 4 {
		t.Errorf("After 5th ACK, expected cwnd to be 4, got %d", handler.cwnd[addr])
	}
	if handler.cAvoidanceAcc[addr] != 0 {
		t.Errorf("After 5th ACK, expected accumulator to be reset to 0, got %d", handler.cAvoidanceAcc[addr])
	}
}

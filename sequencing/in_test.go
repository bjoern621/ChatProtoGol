package sequencing

import (
	"encoding/binary"
	"net"
	"net/netip"
	"testing"

	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/sock"
)

type mockSocket struct {
	addr netip.Addr
}

func (m *mockSocket) MustGetLocalAddress() netip.AddrPort {
	return netip.AddrPortFrom(m.addr, 1234)
}
func (m *mockSocket) GetLocalAddress() (netip.AddrPort, error) {
	return netip.AddrPortFrom(m.addr, 1234), nil
}
func (m *mockSocket) SendTo(addr *net.UDPAddr, data []byte) error { return nil }
func (m *mockSocket) Open(ipv4addr net.IP) (*net.UDPAddr, error)  { return nil, nil }
func (m *mockSocket) Close() error                                { return nil }
func (m *mockSocket) Subscribe() chan *sock.Packet                { return nil }

// Helper to create a packet with given src, dst, seqNum
func makePacket(src, dst netip.Addr, seqNum uint32) *pkt.Packet {
	var pktNum [4]byte
	binary.BigEndian.PutUint32(pktNum[:], seqNum)
	return &pkt.Packet{
		Header: pkt.Header{
			SourceAddr: src.As4(),
			DestAddr:   dst.As4(),
			PktNum:     pktNum,
		},
	}
}

func TestIsDuplicatePacket(t *testing.T) {
	local := netip.MustParseAddr("192.0.2.1")
	peer := netip.MustParseAddr("192.0.2.2")
	h := NewIncomingPktNumHandler(&mockSocket{addr: local})

	// Out-of-order packets should not be duplicates until they are in order
	p5 := makePacket(peer, local, 5)
	dup, err := h.IsDuplicatePacket(p5)
	if dup || err != nil {
		t.Errorf("Out-of-order packet should not be duplicate, got dup=%v err=%v", dup, err)
	}

	// Second out-of-order packet (seq 5 again)
	p5Dup := makePacket(peer, local, 5)
	dup, err = h.IsDuplicatePacket(p5Dup)
	if !dup || err != nil {
		t.Errorf("Duplicate packet should be duplicate, got dup=%v err=%v", dup, err)
	}

	// First packet from peer (seq 0)
	p0 := makePacket(peer, local, 0)
	dup, err = h.IsDuplicatePacket(p0)
	if dup || err != nil {
		t.Errorf("First packet should not be duplicate, got dup=%v err=%v", dup, err)
	}

	// Duplicate packet (seq 0 again)
	p0dup := makePacket(peer, local, 0)
	dup, err = h.IsDuplicatePacket(p0dup)
	if !dup || err != nil {
		t.Errorf("Duplicate packet should be duplicate, got dup=%v err=%v", dup, err)
	}

	// Next-in-order packet (seq 1)
	p1 := makePacket(peer, local, 1)
	dup, err = h.IsDuplicatePacket(p1)
	if dup || err != nil {
		t.Errorf("Next-in-order packet should not be duplicate, got dup=%v err=%v", dup, err)
	}

	// Out-of-order packet (seq 3)
	p3 := makePacket(peer, local, 3)
	dup, err = h.IsDuplicatePacket(p3)
	if dup || err != nil {
		t.Errorf("Out-of-order packet should not be duplicate, got dup=%v err=%v", dup, err)
	}

	// Duplicate out-of-order packet (seq 3 again)
	p3Dup := makePacket(peer, local, 3)
	dup, err = h.IsDuplicatePacket(p3Dup)
	if !dup || err != nil {
		t.Errorf("Duplicate out-of-order packet should be duplicate, got dup=%v err=%v", dup, err)
	}

	// Out-of-order packet (seq 4), should not be duplicate
	p4 := makePacket(peer, local, 4)
	dup, err = h.IsDuplicatePacket(p4)
	if dup || err != nil {
		t.Errorf("Out-of-order packet should not be duplicate, got dup=%v err=%v", dup, err)
	}

	// In-order packet (seq 2), should now advance highest to 5
	p2 := makePacket(peer, local, 2)
	dup, err = h.IsDuplicatePacket(p2)
	if dup || err != nil {
		t.Errorf("In-order packet should not be duplicate, got dup=%v err=%v", dup, err)
	}
	if h.GetHighestContiguousSeqNum(peer) != 5 {
		t.Errorf("Highest contiguous seq num should be 5, got %d", h.GetHighestContiguousSeqNum(peer))
	}

	// Packet too far ahead
	// p100 := makePacket(peer, local, uint32(common.RECEIVER_WINDOW)+10)
	// _, err = h.IsDuplicatePacket(p100)
	// if err == nil {
	// 	t.Errorf("Packet too far ahead should error, got err=nil")
	// }

	// Packet not destined for us
	pWrongDst := makePacket(peer, netip.MustParseAddr("203.0.113.1"), 4)
	_, err = h.IsDuplicatePacket(pWrongDst)
	if err == nil {
		t.Errorf("Packet not destined for us should error")
	}
}

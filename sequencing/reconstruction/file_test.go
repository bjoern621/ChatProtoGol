package reconstruction

import (
	"encoding/binary"
	"net/netip"
	"os"
	"testing"

	"bjoernblessin.de/chatprotogol/pkt"
)

func makePacket(seqNum uint32, payload []byte) *pkt.Packet {
	var pktNum [4]byte
	binary.BigEndian.PutUint32(pktNum[:], seqNum)
	return &pkt.Packet{
		Header: pkt.Header{
			PktNum: pktNum,
		},
		Payload: payload,
	}
}

func TestOnDiskReconstructor_ContiguousWrite(t *testing.T) {
	metaPayload := []byte("testfile_result.bin")
	content1 := []byte("Hello, ")
	content2 := []byte("world!")
	content3 := []byte(" Goodbye.")

	peerAddr := netip.MustParseAddr("10.0.0.2")
	r := NewOnDiskReconstructor(peerAddr)

	// Simulate receiving packets in order: 0 (meta), 1, 2, 3
	r.HandleIncomingFilePacket(makePacket(0, metaPayload))
	r.HandleIncomingFilePacket(makePacket(1, content1))
	r.HandleIncomingFilePacket(makePacket(2, content2))
	r.HandleIncomingFilePacket(makePacket(3, content3))

	filePath, err := r.FinishFilePacketSequence()
	if err != nil {
		t.Fatalf("FinishFilePacketSequence failed: %v", err)
	}

	got, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read reconstructed file: %v", err)
	}
	want := append(append(content1, content2...), content3...)
	if string(got) != string(want) {
		t.Errorf("file contents mismatch.\nGot:  %q\nWant: %q", got, want)
	}
}

func TestOnDiskReconstructor_OutOfOrderWrite(t *testing.T) {
	metaPayload := []byte("testfile_result.bin")
	content1 := []byte("A")
	content2 := []byte("B")
	content3 := []byte("C")

	peerAddr := netip.MustParseAddr("10.0.0.2")
	recon := NewOnDiskReconstructor(peerAddr)

	// Out of order: 0 (meta), 2, 1, 3
	recon.HandleIncomingFilePacket(makePacket(0, metaPayload))
	recon.HandleIncomingFilePacket(makePacket(2, content2))
	recon.HandleIncomingFilePacket(makePacket(1, content1))
	recon.HandleIncomingFilePacket(makePacket(3, content3))

	filePath, err := recon.FinishFilePacketSequence()
	if err != nil {
		t.Fatalf("FinishFilePacketSequence failed: %v", err)
	}

	got, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read reconstructed file: %v", err)
	}
	want := append(append(content1, content2...), content3...)
	if string(got) != string(want) {
		t.Errorf("file contents mismatch (out of order).\nGot:  %q\nWant: %q", got, want)
	}
}

func Test_MissingPackets(t *testing.T) {
	metaPayload := []byte("testfile_result.bin")
	content1 := []byte("X")
	content2 := []byte("Y")

	peerAddr := netip.MustParseAddr("10.0.0.2")
	r := NewOnDiskReconstructor(peerAddr)

	// Simulate missing packet: 0 (meta), 2, 3
	r.HandleIncomingFilePacket(makePacket(0, metaPayload))
	r.HandleIncomingFilePacket(makePacket(2, content1))
	r.HandleIncomingFilePacket(makePacket(3, content2))

	filePath, err := r.FinishFilePacketSequence()
	if err != nil {
		t.Fatalf("FinishFilePacketSequence failed: %v", err)
	}

	got, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read reconstructed file: %v", err)
	}
	want := append(content1, content2...)
	if string(got) != string(want) {
		t.Errorf("file contents mismatch (missing packets).\nGot:  %q\nWant: %q", got, want)
	}
}

func Test_MetadataNotFirstPacket(t *testing.T) {
	metaPayload := []byte("testfile_result.bin")
	content1 := []byte("Hello, ")
	content2 := []byte("world!")
	content3 := []byte(" Goodbye.")

	peerAddr := netip.MustParseAddr("10.0.0.2")

	r := NewOnDiskReconstructor(peerAddr)
	r.HandleIncomingFilePacket(makePacket(1, content1))
	r.HandleIncomingFilePacket(makePacket(3, content3))
	r.HandleIncomingFilePacket(makePacket(0, metaPayload))
	r.HandleIncomingFilePacket(makePacket(2, content2))

	filePath, err := r.FinishFilePacketSequence()
	if err != nil {
		t.Fatalf("FinishFilePacketSequence failed: %v", err)
	}
	got, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read reconstructed file: %v", err)
	}
	want := append(append(content1, content2...), content3...)
	if string(got) != string(want) {
		t.Errorf("file contents mismatch (metadata not first).\nGot:  %q\nWant: %q", got, want)
	}
}

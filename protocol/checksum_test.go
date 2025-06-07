package protocol

import (
	"bytes"
	"testing"
)

func TestSetChecksum(t *testing.T) {
	tests := []struct {
		name     string
		packet   *Packet
		expected [2]byte
	}{
		{
			name: "Very Short Packet",
			packet: &Packet{
				Header: Header{
					SourceAddr: [4]byte{0x12, 0x34, 0x56, 0x78},
					DestAddr:   [4]byte{0x9a, 0xbc, 0xde, 0xf0},
				},
			},
			expected: [2]byte{0x1d, 0xa6},
		},
		{
			name: "Longer Packet",
			packet: &Packet{
				Header: Header{
					SourceAddr: [4]byte{0xc0, 0xa8, 0xae, 0x80},
					DestAddr:   [4]byte{0xc0, 0xa8, 0xae, 0x01},
					Control:    0x0,
					TTL:        0x6,
					SeqNum:     [4]byte{0x0, 0x26, 0x11, 0x5c},
				},
				Payload: []byte{0xdc, 0xba, 0x28, 0xd5, 0x41, 0xda, 0x64, 0xe8, 0x6a, 0x10, 0x80, 0x18, 0x1, 0xfe, 0x0, 0x0, 0x0, 0x0, 0x1, 0x01, 0x8, 0x0a, 0x5c, 0x86, 0xc6, 0xf8, 0xbd, 0x62, 0xe3, 0x6f, 0x6c, 0x69, 0x64, 0x6f, 0x72, 0x0a},
			},
			expected: [2]byte{0x67, 0xea},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var packetBefore *Packet
			packetBefore = &Packet{
				Header:  tt.packet.Header,
				Payload: make([]byte, len(tt.packet.Payload)),
			}
			copy(packetBefore.Payload, tt.packet.Payload)

			SetChecksum(tt.packet)

			if tt.packet.Header.Checksum != tt.expected {
				t.Errorf("SetChecksum() = %04X, expected %04X", tt.packet.Header.Checksum, tt.expected)
			}

			// Check that packet is not modified except for the checksum
			if packetBefore.Header.SourceAddr != tt.packet.Header.SourceAddr ||
				packetBefore.Header.DestAddr != tt.packet.Header.DestAddr ||
				packetBefore.Header.Control != tt.packet.Header.Control ||
				packetBefore.Header.TTL != tt.packet.Header.TTL ||
				packetBefore.Header.SeqNum != tt.packet.Header.SeqNum ||
				!bytes.Equal(packetBefore.Payload, tt.packet.Payload) {
				t.Errorf("SetChecksum() modified packet unexpectedly (Payload not printed!):\n - before = %v\n - after  = %v", packetBefore, tt.packet)
			}
		})
	}
}

func TestVerifyChecksum(t *testing.T) {
	tests := []struct {
		name     string
		packet   *Packet
		expected bool
	}{
		{
			name: "Valid Checksum Short Packet",
			packet: &Packet{
				Header: Header{
					SourceAddr: [4]byte{0x12, 0x34, 0x56, 0x78},
					DestAddr:   [4]byte{0x9a, 0xbc, 0xde, 0xf0},
					Checksum:   [2]byte{0x1d, 0xa6},
				},
			},
			expected: true,
		},
		{
			name: "Valid Checksum Longer Packet",
			packet: &Packet{
				Header: Header{
					SourceAddr: [4]byte{0xc0, 0xa8, 0xae, 0x80},
					DestAddr:   [4]byte{0xc0, 0xa8, 0xae, 0x01},
					Control:    0x0,
					TTL:        0x6,
					SeqNum:     [4]byte{0x0, 0x26, 0x11, 0x5c},
					Checksum:   [2]byte{0x67, 0xea},
				},
				Payload: []byte{0xdc, 0xba, 0x28, 0xd5, 0x41, 0xda, 0x64, 0xe8, 0x6a, 0x10, 0x80, 0x18, 0x1, 0xfe, 0x0, 0x0, 0x0, 0x0, 0x1, 0x01, 0x8, 0x0a, 0x5c, 0x86, 0xc6, 0xf8, 0xbd, 0x62, 0xe3, 0x6f, 0x6c, 0x69, 0x64, 0x6f, 0x72, 0x0a},
			},
			expected: true,
		},
		{
			name: "Invalid Checksum",
			packet: &Packet{
				Header: Header{
					SourceAddr: [4]byte{0x12, 0x34, 0x56, 0x78},
					DestAddr:   [4]byte{0x9a, 0xbc, 0xde, 0xf0},
					Checksum:   [2]byte{0xab, 0xcd}, // Invalid checksum
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VerifyChecksum(tt.packet)
			if result != tt.expected {
				t.Errorf("VerifyChecksum() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

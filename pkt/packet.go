package pkt

import (
	"encoding/binary"
	"errors"
	"fmt"

	"bjoernblessin.de/chatprotogol/util/assert"
)

// Header represents the protocol packet header structure.
// Format:
//
//	+--------+--------+--------+--------+--------+--------+--------+--------+
//	|                                                                       |
//	|                     Source IPv4 Address (32 bits)                     |
//	|                                                                       |
//	+--------+--------+--------+--------+--------+--------+--------+--------+
//	|                                                                       |
//	|                  Destination IPv4 Address (32 bits)                   |
//	|                                                                       |
//	+--------+--------+--------+--------+--------+--------+--------+--------+
//	|  Msg   |  Last  |  Team  |        |                                   |
//	|  Type  |  Bit   |   ID   |  TTL   |        Checksum (16 bits)         |
//	|(4 bits)|(1 bit) |(3 bits)|(8 bits)|                                   |
//	+--------+--------+--------+--------+--------+--------+--------+--------+
//	|                                                                       |
//	|                   Packet Number (32 bits)                             |
//	|                                                                       |
//	+--------+--------+--------+--------+--------+--------+--------+--------+
//
// Total size: 16 bytes (128 bits)
type Header struct {
	SourceAddr [4]byte // Source IP address (32 bits)
	DestAddr   [4]byte // Destination IP address (32 bits)
	Control    byte    // Control byte containing: Message Type (4 bits), Last Bit (1 bit), Team ID (3 bits)
	TTL        byte    // Time to live (8 bits)
	Checksum   [2]byte // Checksum (16 bits)
	PktNum     [4]byte // Packet number (32 bits)
}

// Payload represents the data carried by the packet.
type Payload []byte

type Packet struct {
	Header  Header
	Payload Payload
}

const (
	MsgTypeConnect        = 0x0
	MsgTypeDisconnect     = 0x1
	MsgTypeDD             = 0x2
	MsgTypeLSA            = 0x3
	MsgTypeChatMessage    = 0x4
	MsgTypeFileTransfer   = 0x5
	MsgTypeAcknowledgment = 0x6
	MsgTypeFinish         = 0x7
)

func ParsePacket(data []byte) (*Packet, error) {
	if len(data) < 16 {
		return &Packet{}, errors.New("data length is less than 16 bytes, this is shorter than the header size, invalid packet")
	}

	header := Header{
		SourceAddr: [4]byte{data[0], data[1], data[2], data[3]},
		DestAddr:   [4]byte{data[4], data[5], data[6], data[7]},
		Control:    data[8],
		TTL:        data[9],
		Checksum:   [2]byte{data[10], data[11]},
		PktNum:     [4]byte{data[12], data[13], data[14], data[15]},
	}

	payload := make(Payload, len(data)-16)
	copy(payload, data[16:])

	return &Packet{
		Header:  header,
		Payload: payload,
	}, nil
}

// ToByteArray serializes the Packet struct into a byte array.
// Makes a complete copy of all packet data into a new byte slice.
// Returns a byte array containing the header (16 bytes) followed by the payload.
func (p *Packet) ToByteArray() []byte {
	data := make([]byte, 0, 16+len(p.Payload))
	data = append(data, p.Header.SourceAddr[:]...)
	data = append(data, p.Header.DestAddr[:]...)
	data = append(data, p.Header.Control)
	data = append(data, p.Header.TTL)
	data = append(data, p.Header.Checksum[:]...)
	data = append(data, p.Header.PktNum[:]...)
	data = append(data, p.Payload...)

	return data
}

func (p *Packet) IsLast() bool {
	return p.Header.Control&0b1000 != 0
}

func (p *Packet) GetMessageType() byte {
	return p.Header.Control & 0xF0 >> 4
}

func (p *Packet) GetTeamID() byte {
	return p.Header.Control & 0b111
}

// MakeControlByte creates a control byte for a message packet.
// The control byte is structured as follows:
// - Bits 0-3: Message type (4 bits)
// - Bit 4: Last bit flag (1 bit)
// - Bits 5-7: Team ID (3 bits)
func MakeControlByte(msgType byte, lastBit bool, teamID byte) byte {
	assert.Assert(teamID <= 0b111, "teamID must be 3 bits (0-7)")
	assert.Assert(msgType <= 0b1111, "msgType must be 4 bits (0-15)")

	controlByte := msgType << 4
	if lastBit {
		controlByte |= 0b1000
	}
	controlByte |= teamID

	return controlByte
}

func (p *Packet) String() string {
	return "{ " +
		fmt.Sprintf("Src:%d.%d.%d.%d ", p.Header.SourceAddr[0], p.Header.SourceAddr[1], p.Header.SourceAddr[2], p.Header.SourceAddr[3]) +
		fmt.Sprintf("Dest:%d.%d.%d.%d ", p.Header.DestAddr[0], p.Header.DestAddr[1], p.Header.DestAddr[2], p.Header.DestAddr[3]) +
		fmt.Sprintf("Type:0x%X ", p.GetMessageType()) +
		fmt.Sprintf("Last:%t ", p.IsLast()) +
		fmt.Sprintf("Team:%d ", p.GetTeamID()) +
		fmt.Sprintf("TTL:%d ", p.Header.TTL) +
		fmt.Sprintf("Chksum:0x%04X ", p.Header.Checksum) +
		fmt.Sprintf("PktNum:%d ", binary.BigEndian.Uint32(p.Header.PktNum[:])) +
		"}"
}

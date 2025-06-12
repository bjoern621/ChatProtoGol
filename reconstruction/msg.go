// Package reconstruction handles out-of-order and missing seq nums packets.
// It can buffer "future" packets (packets that are received but can't be used yet).
// It is not responsible for detecting duplicates, that is handled in the sequencing package.
package reconstruction

import (
	"encoding/binary"
	"net/netip"
	"slices"

	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/util/assert"
)

type buffer struct {
	lastBit struct {
		received bool    // Whether the last bit of the packet sequence has been received
		seqNum   [4]byte // The sequence number of the packet that had the last bit set
	}
	payloads map[[4]byte]pkt.Payload // Maps sequence numbers to payloads
}

var (
	payloadBuffer map[netip.Addr]*buffer = make(map[netip.Addr]*buffer) // Maps Peer to a map of buffer information
)

// ClearPayloadBuffer clears the payload buffer for a specific peer.
// This should be called when the peer is no longer needed, such as after a disconnect.
func ClearPayloadBuffer(peerAddr netip.Addr) {
	delete(payloadBuffer, peerAddr)
}

// HandleIncomingMsgPacket processes an incoming message packet.
// It checks if the packet is the last in a sequence and whether all parts of the message have been received.
// If the message is complete, it returns the complete message and a flag indicating readiness.
// The local buffer is cleared after returning the complete message, so the returned message should be copied if needed later.
func HandleIncomingMsgPacket(packet *pkt.Packet, sourcePeerAddr netip.Addr) (completeMsg []byte, isReady bool) {
	if _, exists := payloadBuffer[sourcePeerAddr]; !exists {
		// Received first packet of a sequence from this peer
		payloadBuffer[sourcePeerAddr] = &buffer{
			payloads: make(map[[4]byte]pkt.Payload),
		}
	}

	payloadBuffer[sourcePeerAddr].payloads[packet.Header.PktNum] = packet.Payload

	if packet.IsLast() {
		payloadBuffer[sourcePeerAddr].lastBit.received = true
	}

	isMessageComplete := payloadBuffer[sourcePeerAddr].lastBit.received && binary.BigEndian.Uint32(payloadBuffer[sourcePeerAddr].lastBit.seqNum[:]) <= sequencing.GetHighestContiguousSeqNum(sourcePeerAddr)

	if !isMessageComplete {
		// The message is not complete yet, we need to wait for more parts
		return nil, false
	}

	sortedSeqNums := []uint32{}
	for seqNum := range payloadBuffer[sourcePeerAddr].payloads {
		sortedSeqNums = append(sortedSeqNums, binary.BigEndian.Uint32(seqNum[:]))
	}
	slices.Sort(sortedSeqNums)

	for _, seqNum := range sortedSeqNums {
		var seqNumBytes [4]byte
		binary.BigEndian.PutUint32(seqNumBytes[:], seqNum)
		payload, exists := payloadBuffer[sourcePeerAddr].payloads[seqNumBytes]
		assert.Assert(exists, "Payload should exist for sequence number %d", seqNum)

		completeMsg = append(completeMsg, payload...)
	}

	ClearPayloadBuffer(sourcePeerAddr)

	return completeMsg, true
}

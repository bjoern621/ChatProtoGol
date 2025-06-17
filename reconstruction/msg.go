// Package reconstruction handles out-of-order and missing pkt nums packets.
// It can buffer "future" packets (packets that are received but can't be used yet).
// It is not responsible for detecting duplicates, that is handled in the sequencing package.
package reconstruction

import (
	"encoding/binary"
	"net/netip"
	"slices"

	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/util/assert"
)

type buffer struct {
	lastBit struct {
		received bool    // Whether the last bit of the packet sequence has been received
		seqNum   [4]byte // The sequence number of the packet that had the last bit set
	}
	payloads map[[4]byte]pkt.Payload // Maps sequence numbers to payloads
}

// ClearPayloadBuffer clears the payload buffer for a specific host.
// This should be called when the host is no longer needed, such as after a "full" disconnect.
func (r *PktSequenceReconstructor) ClearPayloadBuffer(addr netip.Addr) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.clearPayloadBuffer(addr)
}

func (r *PktSequenceReconstructor) clearPayloadBuffer(addr netip.Addr) {
	delete(r.payloadBuffer, addr)
}

// HandleIncomingMsgPacket processes an incoming message packet.
// It checks if the packet is the last in a sequence and whether all parts of the message have been received.
// If the message is complete, it returns the complete message and a flag indicating readiness.
// The local buffer is cleared after returning the complete message, so the returned message should be copied if needed later.
func (r *PktSequenceReconstructor) HandleIncomingMsgPacket(packet *pkt.Packet, sourceAddr netip.Addr) (completeMsg []byte, isReady bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.payloadBuffer[sourceAddr]; !exists {
		// Received first packet of a sequence from this host
		r.payloadBuffer[sourceAddr] = &buffer{
			payloads: make(map[[4]byte]pkt.Payload),
		}
	}

	r.payloadBuffer[sourceAddr].payloads[packet.Header.PktNum] = packet.Payload

	if packet.IsLast() {
		r.payloadBuffer[sourceAddr].lastBit.received = true
	}

	isMessageComplete := r.payloadBuffer[sourceAddr].lastBit.received && binary.BigEndian.Uint32(r.payloadBuffer[sourceAddr].lastBit.seqNum[:]) <= r.sequencing.GetHighestContiguousSeqNum(sourceAddr)

	if !isMessageComplete {
		// The message is not complete yet, we need to wait for more parts
		return nil, false
	}

	sortedSeqNums := []uint32{}
	for seqNum := range r.payloadBuffer[sourceAddr].payloads {
		sortedSeqNums = append(sortedSeqNums, binary.BigEndian.Uint32(seqNum[:]))
	}
	slices.Sort(sortedSeqNums)

	for _, seqNum := range sortedSeqNums {
		var seqNumBytes [4]byte
		binary.BigEndian.PutUint32(seqNumBytes[:], seqNum)
		payload, exists := r.payloadBuffer[sourceAddr].payloads[seqNumBytes]
		assert.Assert(exists, "Payload should exist for packet number %d", seqNum)

		completeMsg = append(completeMsg, payload...)
	}

	r.clearPayloadBuffer(sourceAddr)

	return completeMsg, true
}

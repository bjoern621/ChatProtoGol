// Package reconstruction handles out-of-order and missing pkt nums packets.
// It can buffer "future" packets (packets that are received but can't be used yet).
// It is not responsible for detecting duplicates, that is handled in the sequencing package.
package reconstruction

import (
	"encoding/binary"
	"errors"
	"slices"

	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/util/assert"
)

type InMemoryReconstructor struct {
	bufferedPayloads map[[4]byte]pkt.Payload
}

// NewInMemoryReconstructor creates a new InMemoryReconstructor instance.
func NewInMemoryReconstructor() *InMemoryReconstructor {
	return &InMemoryReconstructor{
		bufferedPayloads: make(map[[4]byte]pkt.Payload),
	}
}

// HandleIncomingMsgPacket processes an incoming message packet.
// It stores the payload in the reconstruction buffer.
// The buffer can be read later using finishMsgPacketSequence.
func (r *InMemoryReconstructor) HandleIncomingMsgPacket(packet *pkt.Packet) {
	r.bufferedPayloads[packet.Header.PktNum] = packet.Payload
}

// FinishMsgPacketSequence completes the current packet sequence for a specific source address.
// The local buffer is cleared after returning the complete message, so the returned message should be copied if needed later.
func (r *InMemoryReconstructor) FinishMsgPacketSequence() (completeMsg []byte, err error) {
	sortedSeqNums := []uint32{}
	for seqNum := range r.bufferedPayloads {
		sortedSeqNums = append(sortedSeqNums, binary.BigEndian.Uint32(seqNum[:]))
	}
	slices.Sort(sortedSeqNums)

	for _, seqNum := range sortedSeqNums {
		var seqNumBytes [4]byte
		binary.BigEndian.PutUint32(seqNumBytes[:], seqNum)
		payload, exists := r.bufferedPayloads[seqNumBytes]
		assert.Assert(exists, "Payload should exist for packet number %d", seqNum)

		completeMsg = append(completeMsg, payload...)
	}

	return completeMsg, nil
}

// GetHighestPktNum returns the highest packet number that has been processed by this reconstructor.
func (r *InMemoryReconstructor) GetHighestPktNum() (uint32, error) {
	if len(r.bufferedPayloads) == 0 {
		return 0, errors.New("no packets buffered")
	}

	var highestPktNum uint32
	for seqNum := range r.bufferedPayloads {
		pktNum := binary.BigEndian.Uint32(seqNum[:])
		if pktNum > highestPktNum {
			highestPktNum = pktNum
		}
	}

	return highestPktNum, nil
}

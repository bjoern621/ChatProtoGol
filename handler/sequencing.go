// This file is concerned about sequencing packets.
// 	sequenceNumber uint32
// It can detect out-of-order packets and handle them accordingly.
// It checks duplicate packets and manages the sequence number for each peer.

// It is NOT concerned about ordering packets.

package handler

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"sync"

	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/socket"
	"bjoernblessin.de/chatprotogol/util/assert"
)

var (
	seqMu         sync.Mutex
	highestSeqNum map[netip.Addr]uint32          = make(map[netip.Addr]uint32)          // Highest contiguous seq num received per peer
	futureSeqNums map[netip.Addr]map[uint32]bool = make(map[netip.Addr]map[uint32]bool) // Out-of-order seq nums > highest
)

// trackNewPacket checks if the packet is a duplicate, and updates sequencing state.
// It uses the sequence number from the packet header to determine if it has already been received.
// This means it should only be used on packets with an UNIQUE sequence number (i.e., packets that have DestAddr == socket.GetLocalAddress() and have message types that provide sequence numbers).
// Returns true if the packet is a duplicate (already received), false otherwise.
func trackNewPacket(packet *pkt.Packet) (bool, error) {
	assert.Assert(netip.AddrFrom4(packet.Header.DestAddr) == socket.GetLocalAddress().AddrPort().Addr(), "isDuplicatePacket should only be called for packets destined for us")

	seqMu.Lock()
	defer seqMu.Unlock()

	peerAddr := netip.AddrFrom4(packet.Header.SourceAddr)
	seqNum := binary.BigEndian.Uint32(packet.Header.SeqNum[:])

	highest, hasHighest := highestSeqNum[peerAddr]
	if !hasHighest {
		// highestSeqNum[peerAddr] = seqNum
		highestSeqNum[peerAddr] = 0
		return false, nil // First packet from this peer
	}

	if seqNum <= highest {
		// seqNum <= highest, so it's a duplicate
		return true, nil
	} else if seqNum == highest+1 {
		highestSeqNum[peerAddr]++

		// Advance highest if future packets are now contiguous
		for {
			futurePackets, ok := futureSeqNums[peerAddr]
			nextHighestAlreadyReceived := ok && futurePackets[highestSeqNum[peerAddr]+1]

			if !nextHighestAlreadyReceived {
				break
			}

			highestSeqNum[peerAddr]++
			delete(futurePackets, highestSeqNum[peerAddr])
			if len(futurePackets) == 0 {
				delete(futureSeqNums, peerAddr)
			}
			continue
		}

		return false, nil
	} else if seqNum > highest+1 {
		// Out-of-order, store for future

		if seqNum-highest > common.RECEIVE_BUFFER_SIZE {
			return true, errors.New("Received packet with sequence number too far ahead, dropping packet")
		}

		if _, ok := futureSeqNums[peerAddr]; !ok {
			futureSeqNums[peerAddr] = make(map[uint32]bool)
		}
		futureSeqNums[peerAddr][seqNum] = true

		return false, nil
	}

	assert.Never("Unexpected sequence number logic: seqNum=%d, highest=%d", seqNum, highest)
	return true, nil
}

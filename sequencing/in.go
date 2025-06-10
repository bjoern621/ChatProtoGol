// Package sequencing handles the sequencing of packets.
// On the receiving side, it checks if a packet is a duplicate.
// It does not handle re-order packets, but it tracks their sequence numbers to detect duplicates packets.
// On the sending side, it provides a unique sequence number for each peer.
package sequencing

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
	futureSeqNums map[netip.Addr]map[uint32]bool = make(map[netip.Addr]map[uint32]bool) // Out-of-order seq nums > highest, bounded by common.RECEIVE_BUFFER_SIZE
)

// TODO probably doesnt handle wrapping of sequence numbers (e.g., if highestSeqNum is 0xFFFFFFFF and we receive a packet with seqNum 0x00000000)

// IsDuplicatePacket checks if the packet is a duplicate, and updates sequencing state.
// It uses the sequence number from the packet header to determine if it has already been received.
// This means it should only be used on packets with an UNIQUE sequence number (i.e., packets that have DestAddr == socket.GetLocalAddress() and have message types that provide sequence numbers).
// Returns true if the packet is a duplicate (already received), false otherwise.
// Errors if the sequence number is too far ahead (more than common.RECEIVE_BUFFER_SIZE).
func IsDuplicatePacket(packet *pkt.Packet) (bool, error) {
	assert.Assert(netip.AddrFrom4(packet.Header.DestAddr) == socket.GetLocalAddress().AddrPort().Addr(), "isDuplicatePacket should only be called for packets destined for us")

	seqMu.Lock()
	defer seqMu.Unlock()

	peerAddr := netip.AddrFrom4(packet.Header.SourceAddr)
	seqNum := binary.BigEndian.Uint32(packet.Header.SeqNum[:])

	highest, hasHighest := highestSeqNum[peerAddr]
	if !hasHighest {
		// highestSeqNum[peerAddr] = seqNum
		highestSeqNum[peerAddr] = 0 // TODO may not be correct, what if the first packet has a seqNum like 10?
		return false, nil           // First packet from this peer
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
		// Out-of-order, store seq num for future

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

func GetHighestContiguousSeqNum(peerAddr netip.Addr) uint32 {
	seqMu.Lock()
	defer seqMu.Unlock()

	highest, exists := highestSeqNum[peerAddr]
	if !exists {
		return 0
	}

	return highest
}

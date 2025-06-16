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
	"bjoernblessin.de/chatprotogol/sock"
	"bjoernblessin.de/chatprotogol/util/assert"
)

type IncomingPktNumHandler struct {
	seqMu         sync.Mutex
	highestPktNum map[netip.Addr]uint32          // Highest contiguous seq num received per peer
	futurePktNums map[netip.Addr]map[uint32]bool // Out-of-order seq nums > highest, bounded by common.RECEIVE_BUFFER_SIZE
	socket        sock.Socket
}

func NewIncomingPktNumHandler(socket sock.Socket) *IncomingPktNumHandler {
	return &IncomingPktNumHandler{
		highestPktNum: make(map[netip.Addr]uint32),
		futurePktNums: make(map[netip.Addr]map[uint32]bool),
		socket:        socket,
	}
}

func (h *IncomingPktNumHandler) ClearIncomingPacketNumbers(peerAddr netip.Addr) {
	h.seqMu.Lock()
	defer h.seqMu.Unlock()

	delete(h.highestPktNum, peerAddr)

	delete(h.futurePktNums, peerAddr)
}

// IsDuplicatePacket checks if the packet is a duplicate, and updates sequencing state.
// It uses the packet number from the packet header to determine if it has already been received.
// This means it should only be used on packets with an UNIQUE packet number (i.e., packets that have DestAddr == socket.GetLocalAddress() and have message types that provide packet numbers).
// Returns true if the packet is a duplicate (already received), false otherwise.
// Errors if the packet number is too far ahead (more than common.RECEIVE_BUFFER_SIZE).
func (h *IncomingPktNumHandler) IsDuplicatePacket(packet *pkt.Packet) (bool, error) {
	assert.Assert(netip.AddrFrom4(packet.Header.DestAddr) == h.socket.MustGetLocalAddress().Addr(), "isDuplicatePacket should only be called for packets destined for us")

	h.seqMu.Lock()
	defer h.seqMu.Unlock()

	peerAddr := netip.AddrFrom4(packet.Header.SourceAddr)
	seqNum := binary.BigEndian.Uint32(packet.Header.PktNum[:])

	highest, hasHighest := h.highestPktNum[peerAddr]
	if !hasHighest {
		// highestSeqNum[peerAddr] = seqNum
		h.highestPktNum[peerAddr] = 0 // TODO may not be correct, what if the first packet has a seqNum like 10?
		return false, nil             // First packet from this peer
	}

	if seqNum <= highest {
		// seqNum <= highest, so it's a duplicate
		return true, nil
	} else if seqNum == highest+1 {
		h.highestPktNum[peerAddr]++

		// Advance highest if future packets are now contiguous
		for {
			futurePackets, ok := h.futurePktNums[peerAddr]
			nextHighestAlreadyReceived := ok && futurePackets[h.highestPktNum[peerAddr]+1]

			if !nextHighestAlreadyReceived {
				break
			}

			h.highestPktNum[peerAddr]++
			delete(futurePackets, h.highestPktNum[peerAddr])
			if len(futurePackets) == 0 {
				delete(h.futurePktNums, peerAddr)
			}
			continue
		}

		return false, nil
	} else if seqNum > highest+1 {
		// Out-of-order, store seq num for future

		if seqNum-highest > common.RECEIVE_BUFFER_SIZE {
			return true, errors.New("Received packet with sequence number too far ahead, dropping packet")
		}

		if _, ok := h.futurePktNums[peerAddr]; !ok {
			h.futurePktNums[peerAddr] = make(map[uint32]bool)
		}
		h.futurePktNums[peerAddr][seqNum] = true

		return false, nil
	}

	assert.Never("Unexpected sequence number logic: seqNum=%d, highest=%d", seqNum, highest)
	return true, nil
}

func (h *IncomingPktNumHandler) GetHighestContiguousSeqNum(peerAddr netip.Addr) uint32 {
	h.seqMu.Lock()
	defer h.seqMu.Unlock()

	highest, exists := h.highestPktNum[peerAddr]
	if !exists {
		return 0
	}

	return highest
}

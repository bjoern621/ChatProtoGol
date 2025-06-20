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
	highestPktNum map[netip.Addr]int64          // Highest contiguous seq num received per peer; int64 to allow for negative numbers
	futurePktNums map[netip.Addr]map[int64]bool // Out-of-order seq nums > highest, bounded by common.RECEIVE_BUFFER_SIZE
	socket        sock.Socket
}

func NewIncomingPktNumHandler(socket sock.Socket) *IncomingPktNumHandler {
	return &IncomingPktNumHandler{
		highestPktNum: make(map[netip.Addr]int64),
		futurePktNums: make(map[netip.Addr]map[int64]bool),
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
// Errors if the packet number is too far ahead (more than common.RECEIVE_BUFFER_SIZE)
// or if the packet is not destined for us (i.e., the source address does not match the local address).
func (h *IncomingPktNumHandler) IsDuplicatePacket(packet *pkt.Packet) (bool, error) {
	if netip.AddrFrom4(packet.Header.DestAddr) != h.socket.MustGetLocalAddress().Addr() {
		return false, errors.New("packet is not destined for us, cannot check for duplicates. header destAddr: " + netip.AddrFrom4(packet.Header.DestAddr).String())
	}

	h.seqMu.Lock()
	defer h.seqMu.Unlock()

	peerAddr := netip.AddrFrom4(packet.Header.SourceAddr)
	seqNum32 := binary.BigEndian.Uint32(packet.Header.PktNum[:])

	seqNum := int64(seqNum32)

	highest, hasHighest := h.highestPktNum[peerAddr]
	if !hasHighest {
		highest = -1
	}

	if seqNum <= highest {
		return true, nil
	} else if seqNum == highest+1 {
		h.highestPktNum[peerAddr] = seqNum

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
			h.futurePktNums[peerAddr] = make(map[int64]bool)
		}

		if _, exists := h.futurePktNums[peerAddr][seqNum]; exists {
			// Already stored this seq num, so it's a duplicate
			return true, nil
		}

		h.futurePktNums[peerAddr][seqNum] = true

		return false, nil
	}

	assert.Never("Unexpected sequence number logic: seqNum=%d, highest=%d", seqNum, highest)
	return true, nil
}

func (h *IncomingPktNumHandler) GetHighestContiguousSeqNum(peerAddr netip.Addr) int64 {
	h.seqMu.Lock()
	defer h.seqMu.Unlock()

	highest, exists := h.highestPktNum[peerAddr]
	if !exists {
		return -1
	}

	return highest
}

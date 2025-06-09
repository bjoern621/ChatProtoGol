package handler

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"slices"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/socket"
	"bjoernblessin.de/chatprotogol/util/assert"
	"bjoernblessin.de/chatprotogol/util/logger"
)

type buffer struct {
	lastBit struct {
		received bool    // Whether the last bit of the packet sequence has been received
		seqNum   [4]byte // The sequence number of the packet that had the last bit set
	}
	payloads map[[4]byte]pkt.Payload // Maps sequence numbers to payloads
}

var (
	payloadBuffer map[*connection.Peer]*buffer = make(map[*connection.Peer]*buffer) // Maps Peer to a map of buffer information
)

// ClearPayloadBuffer clears the payload buffer for a specific peer.
// This should be called when the peer is no longer needed, such as after a disconnect.
func ClearPayloadBuffer(peer *connection.Peer) {
	delete(payloadBuffer, peer)
}

func handleMsg(packet *pkt.Packet) {
	destAddr := netip.AddrFrom4(packet.Header.DestAddr)

	if destAddr == socket.GetLocalAddress().AddrPort().Addr() {
		// The message is for us

		duplicate, dupErr := isDuplicatePacket(packet)
		if dupErr != nil {
			return
		} else if duplicate {
			peer, exists := connection.GetPeer(netip.AddrFrom4(packet.Header.SourceAddr))
			assert.Assert(exists, "Duplicate packet should have a peer")
			peer.SendAcknowledgment(packet.Header.SeqNum)
			return
		}

		logger.Infof("MSG RECEIVED %v %d", packet.Header.SourceAddr, packet.Header.SeqNum)

		sourcePeer, found := connection.GetPeer(netip.AddrFrom4(packet.Header.SourceAddr))
		if !found {
			logger.Warnf("No peer found for source address %s, can't handle message", packet.Header.SourceAddr)
			return
		}

		sourcePeer.SendAcknowledgment(packet.Header.SeqNum)

		// Handle reconstruction of payloads (missing seqnums, out-of-order)

		if _, exists := payloadBuffer[sourcePeer]; !exists {
			// Received first packet of a sequence from this peer
			payloadBuffer[sourcePeer] = &buffer{
				payloads: make(map[[4]byte]pkt.Payload),
			}
		}

		payloadBuffer[sourcePeer].payloads[packet.Header.SeqNum] = packet.Payload

		if packet.IsLast() {
			payloadBuffer[sourcePeer].lastBit.received = true
		}

		isMessageComplete := payloadBuffer[sourcePeer].lastBit.received && binary.BigEndian.Uint32(payloadBuffer[sourcePeer].lastBit.seqNum[:]) <= getHighestContiguousSeqNum(sourcePeer.Address)

		if !isMessageComplete {
			// The message is not complete yet, we need to wait for more parts
			return
		}

		sortedSeqNums := []uint32{}
		for seqNum := range payloadBuffer[sourcePeer].payloads {
			sortedSeqNums = append(sortedSeqNums, binary.BigEndian.Uint32(seqNum[:]))
		}
		slices.Sort(sortedSeqNums)

		fmt.Printf("MSG %v: ", sourcePeer.Address)

		for _, seqNum := range sortedSeqNums {
			var seqNumBytes [4]byte
			binary.BigEndian.PutUint32(seqNumBytes[:], seqNum)
			payload, exists := payloadBuffer[sourcePeer].payloads[seqNumBytes]
			assert.Assert(exists, "Payload should exist for sequence number %d", seqNum)

			fmt.Printf("%s ", payload) // TODO splitted codepoints
		}

		fmt.Println()

		delete(payloadBuffer, sourcePeer)
	} else {
		// The message is for another peer

		destPeer, found := connection.GetPeer(destAddr)
		if !found {
			logger.Warnf("No peer found for destination address %s, can't forward", destAddr)
			return
		}

		destPeer.Forward(packet)
		return
	}
}

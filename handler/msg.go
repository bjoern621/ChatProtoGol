package handler

import (
	"net/netip"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/socket"
	"bjoernblessin.de/chatprotogol/util/assert"
	"bjoernblessin.de/chatprotogol/util/logger"
)

var (
	payloadBuffer map[*connection.Peer]map[[4]byte]pkt.Payload = make(map[*connection.Peer]map[[4]byte]pkt.Payload) // Maps Peer to a map of sequence numbers to payloads
)

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

		// handle packet (may have missing seqnum, be in wrong order, etc.) BUT NO DUPLICATE POSSIBLE HERE

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

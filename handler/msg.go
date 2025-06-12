package handler

import (
	"fmt"
	"net/netip"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/reconstruction"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/socket"
	"bjoernblessin.de/chatprotogol/util/assert"
	"bjoernblessin.de/chatprotogol/util/logger"
)

func handleMsg(packet *pkt.Packet) {
	destAddr := netip.AddrFrom4(packet.Header.DestAddr)

	if destAddr == socket.GetLocalAddress().AddrPort().Addr() {
		// The message is for us

		duplicate, dupErr := sequencing.IsDuplicatePacket(packet)
		if dupErr != nil {
			return
		} else if duplicate {
			peer, exists := connection.GetPeer(netip.AddrFrom4(packet.Header.SourceAddr))
			assert.Assert(exists, "Duplicate packet should have a peer")
			peer.SendAcknowledgment(packet.Header.PktNum)
			return
		}

		logger.Infof("MSG RECEIVED %v %d", packet.Header.SourceAddr, packet.Header.PktNum)

		sourcePeer, found := connection.GetPeer(netip.AddrFrom4(packet.Header.SourceAddr))
		if !found {
			logger.Warnf("No peer found for source address %s, can't handle message", packet.Header.SourceAddr)
			return
		}

		sourcePeer.SendAcknowledgment(packet.Header.PktNum)

		// Handle reconstruction of payloads (missing seqnums, out-of-order)

		completeMsg, isReady := reconstruction.HandleIncomingMsgPacket(packet, sourcePeer.Address)
		if !isReady {
			return
		}

		fmt.Printf("MSG %v: %s\n", sourcePeer.Address, completeMsg)
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

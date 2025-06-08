package handler

import (
	"net"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/util/logger"
)

func handleConnect(packet *pkt.Packet, sourceAddr *net.UDPAddr) {
	logger.Infof("CONN FROM %v", packet.Header.SourceAddr)

	senderAddrPort := sourceAddr.AddrPort()
	senderAddr := sourceAddr.AddrPort().Addr()

	peer := connection.NewPeer(senderAddr)

	connection.AddRoutingEntry(senderAddr, 1, senderAddrPort)

	peer.SendAcknowledgment(packet.Header.SeqNum)

	payload := connection.FormatRoutingTableForPayload()

	err := connection.SendNewAll(pkt.MsgTypeRoutingTableUpdate, true, payload, connection.GetAllPeers())
	if err != nil {
		logger.Warnf("Failed to send routing table update to %v: %v", senderAddrPort, err)
		return
	}
}

func handleDisconnect(packet *pkt.Packet, sourceAddr *net.UDPAddr) {
	logger.Infof("DISCO FROM %v", packet.Header.SourceAddr)
	// Handle disconnect logic here
}

func handleRoutingTableUpdate(packet *pkt.Packet, sourceAddr *net.UDPAddr) {
	logger.Infof("ROUTING FROM %v", packet.Header.SourceAddr)

	rt, err := connection.ParseRoutingTableFromPayload(packet.Payload, sourceAddr.AddrPort())
	if err != nil {
		logger.Warnf("Failed to parse routing table from payload: %v", err)
		return
	}

	connection.UpdateRoutingTable(rt, sourceAddr.AddrPort())

	peer, exists := connection.GetPeer(sourceAddr.AddrPort().Addr())
	if !exists {
		logger.Warnf("Received routing table update from unknown peer %v", sourceAddr.AddrPort().Addr())
		return
	}
	peer.SendAcknowledgment(packet.Header.SeqNum)

	// TODO resend / forwards routing table to other peers
}

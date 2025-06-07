package handler

import (
	"net"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/util/logger"
)

func handleConnect(packet *pkt.Packet, sourceAddr *net.UDPAddr) {
	logger.Infof("Connect request received from %v", packet.Header.SourceAddr)

	senderAddrPort := sourceAddr.AddrPort()
	senderAddr := sourceAddr.AddrPort().Addr()

	connection.NewPeer(senderAddr)

	connection.AddRoutingEntry(senderAddr, 1, senderAddrPort)

	payload := connection.FormatRoutingTableForPayload()

	err := connection.SendAll(pkt.MsgTypeRoutingTableUpdate, true, payload, connection.GetAllPeers())
	if err != nil {
		logger.Warnf("Failed to send routing table update to %v: %v", senderAddrPort, err)
		return
	}
}

func handleDisconnect(packet *pkt.Packet, sourceAddr *net.UDPAddr) {
	logger.Infof("Disconnect request received from %v", packet.Header.SourceAddr)
	// Handle disconnect logic here
}

func handleRoutingTableUpdate(packet *pkt.Packet, sourceAddr *net.UDPAddr) {
	logger.Infof("Routing table update received from %v", packet.Header.SourceAddr)

	// connection.UpdateRoutingTable()
}

package cmd

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/sock"
	"bjoernblessin.de/chatprotogol/util/assert"
	"bjoernblessin.de/chatprotogol/util/logger"
)

// HandleConnect processes the "connect" command to establish a connection to a specified IP address and port.
func HandleConnect(args []string) {
	if len(args) == 1 {
		if !strings.Contains(args[0], ":") {
			printUsage()
			return
		}

		parts := strings.Split(args[0], ":")

		if len(parts) != 2 {
			printUsage()
			return
		}

		connect(parts[0], parts[1])
	} else if len(args) == 2 {
		connect(args[0], args[1])
	} else {
		printUsage()
	}
}

func connect(ipv4String string, portString string) {
	addr, err := netip.ParseAddr(ipv4String)
	if err != nil {
		fmt.Printf("Invalid IP address: %s\n", ipv4String)
		return
	}

	port, err := strconv.Atoi(portString)
	if err != nil {
		fmt.Printf("Invalid port number: %s\n", portString)
		return
	}

	if !addr.Is4() {
		fmt.Printf("The provided IP address is not a valid IPv4 address: %s\n", ipv4String)
		return
	}

	if isNeighbor, _ := router.IsNeighbor(addr); isNeighbor {
		fmt.Printf("Already connected to %s\n", addr)
		return
	}

	addrPort := netip.AddrPortFrom(addr, uint16(port))

	packet := connection.BuildSequencedPacket(pkt.MsgTypeConnect, true, nil, addr)

	go func() {
		success := <-outSequencing.SubscribeToReceivedAck(packet)
		if success {
			handleConnectAck(addrPort, socket)
		} else {
			logger.Warnf("Acknowledgment for connection request to %s:%d was not received", addr, port)
		}
	}()

	err = connection.SendReliablePacketTo(addrPort, packet)
	if err != nil {
		fmt.Printf("Failed to send connect message: %v\n", err)
		return
	}
}

func printUsage() {
	fmt.Println("Usage: con (<IP address> <port> | <IP address:port>) Example: con 10.0.0.2 8080; con 10.0.0.2:8080")
}

func handleConnectAck(addrPort netip.AddrPort, socket sock.Socket) {
	router.AddNeighbor(addrPort)

	localAddr := socket.MustGetLocalAddress().Addr()

	localLSA, exists := router.GetLSA(localAddr)
	assert.Assert(exists, "Local LSA should exist for the local address")
	connection.FloodLSA(localAddr, localLSA)

	err := connection.SendDD(addrPort)
	if err != nil {
		logger.Warnf("Failed to send database description to %s: %v", addrPort, err)
	}
}

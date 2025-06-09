package cmd

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
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
	peerIP, err := netip.ParseAddr(ipv4String)
	if err != nil {
		fmt.Printf("Invalid IP address: %s\n", ipv4String)
		return
	}

	port, err := strconv.Atoi(portString)
	if err != nil {
		fmt.Printf("Invalid port number: %s\n", portString)
		return
	}

	if !peerIP.Is4() {
		fmt.Printf("The provided IP address is not a valid IPv4 address: %s\n", ipv4String)
		return
	}

	if connection.IsNeighbor(peerIP) {
		fmt.Printf("Already connected to %s\n", peerIP)
		return
	}

	peerAddrPort := netip.AddrPortFrom(peerIP, uint16(port))
	peer := connection.NewPeer(peerAddrPort.Addr())

	payload := connection.FormatRoutingTableForPayload()

	err = peer.SendNewTo(peerAddrPort, pkt.MsgTypeConnect, true, payload)
	if err != nil {
		fmt.Printf("Failed to send connect message: %v\n", err)
		return
	}
}

func printUsage() {
	fmt.Println("Usage: connect (<IP address> <port> | <IP address:port>) Example: con 10.0.0.2 8080; con 10.0.0.2:8080")
}

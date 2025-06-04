package cmd

import (
	"crypto/rand"
	"fmt"
	"net"
	"strconv"

	"bjoernblessin.de/chatprotogol/socket"
)

// HandleConnect processes the "connect" command to establish a connection to a specified IP address and port.
func HandleConnect(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: connect <IP address> <port> Example: connect 10.0.0.2 8080")
		return
	}

	hostIP := net.ParseIP(args[0])
	if hostIP == nil {
		fmt.Printf("Invalid IPv4 address: %s\n", args[0])
		return
	}

	port, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Printf("Invalid port number: %s\n", args[1])
		return
	}

	data := make([]byte, 9500)
	rand.Read(data)

	err = socket.SendTo(&net.UDPAddr{IP: hostIP, Port: port}, data)
	if err != nil {
		fmt.Printf("Failed to send connect message: %v\n", err)
		return
	}
}

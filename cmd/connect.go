package cmd

import (
	"fmt"
	"net"
	"strconv"

	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/protocol"
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
		fmt.Printf("Invalid IP address: %s\n", args[0])
		return
	}

	port, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Printf("Invalid port number: %s\n", args[1])
		return
	}

	ipv4 := hostIP.To4()
	if ipv4 == nil {
		fmt.Printf("The provided IP address is not a valid IPv4 address: %s\n", args[0])
		return
	}

	packet := &protocol.Packet{
		Header: protocol.Header{
			SourceAddr: [4]byte{10, 0, 0, 1},
			DestAddr:   [4]byte{ipv4[0], ipv4[1], ipv4[2], ipv4[3]},
			Control:    protocol.MakeControlByte(protocol.MsgTypeConnect, true, common.TEAM_ID),
			TTL:        common.INITIAL_TTL,
			Checksum:   [2]byte{120, 255},
			SeqNum:     [4]byte{0, 0, 0, 0},
		},
		Payload: []byte{},
	}

	fmt.Printf("Packet to send: %+v\n", packet)

	err = socket.SendTo(&net.UDPAddr{IP: hostIP, Port: port}, packet.ToByteArray())
	if err != nil {
		fmt.Printf("Failed to send connect message: %v\n", err)
		return
	}
}

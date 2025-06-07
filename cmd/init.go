package cmd

import (
	"fmt"
	"net"

	"bjoernblessin.de/chatprotogol/socket"
)

func HandleInit(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: init <IP address> Example: init 10.8.0.6")
		return
	}

	hostIP := net.ParseIP(args[0])
	if hostIP == nil {
		fmt.Printf("Invalid IP address: %s\n", args[0])
		return
	}

	if hostIP.IsUnspecified() {
		fmt.Println("The provided IP address is unspecified. Please provide a valid IP address.")
		return
	}

	ipv4 := hostIP.To4()
	if ipv4 == nil {
		fmt.Printf("The provided IP address is not a valid IPv4 address: %s\n", args[0])
		return
	}

	socket.Close()

	localAddr, err := socket.Open(ipv4)
	if err != nil {
		fmt.Printf("Failed to open UDP socket: %v\n", err.Error())
		return
	}

	fmt.Printf("Listening on %s:%d\n", localAddr.IP, localAddr.Port)
}

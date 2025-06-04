// Package socket manages the UDP socket. The socket can send and receive UDP packets.
// There is only one socket per application. All packets are sent through this socket.
package socket

import (
	"log"
	"net"

	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/util/assert"
	"bjoernblessin.de/chatprotogol/util/logger"
)

var udpSocket *net.UDPConn

// Open opens a UDP socket on all available network interfaces.
// The local port is randomly choosen and returned.
func Open() (int, error) {
	assert.Assert(udpSocket == nil, "UDP socket is already initialized. Call Close() before calling Open() again.")

	socket, err := net.ListenUDP("udp", &net.UDPAddr{Port: 0})
	if err != nil {
		return 0, err
	}

	udpSocket = socket

	go readLoop()

	return socket.LocalAddr().(*net.UDPAddr).Port, nil
}

func readLoop() {
	for {
		buffer := make([]byte, common.UDP_BUFFER_SIZE)
		n, addr, err := udpSocket.ReadFromUDP(buffer)
		if err != nil {
			logger.Warnf("Failed to read from UDP socket: %v", err)
			continue
		}

		log.Printf("[FROM %s %d bytes] %x", addr.String(), n, buffer[:n])
	}
}

// SendTo sends a byte array to the specified address.
// Open() must be called once before using this function.
func SendTo(addr *net.UDPAddr, data []byte) error {
	assert.IsNotNil(udpSocket, "UDP socket is not initialized.")

	n, err := udpSocket.WriteToUDP(data, addr)
	if err != nil {
		return err
	}

	log.Printf("[TO %s %d bytes]\n", addr.String(), n)
	return nil
}

// Package socket manages the UDP socket. The socket can send and receive UDP packets.
// There is only one socket per application. All packets are sent through this socket.
package socket

import (
	"errors"
	"net"

	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/util/assert"
	"bjoernblessin.de/chatprotogol/util/logger"
	"bjoernblessin.de/chatprotogol/util/observer"
)

type Packet struct {
	Addr *net.UDPAddr
	Data []byte
}

var (
	udpSocket        *net.UDPConn
	packetObservable = observer.NewObservable[*Packet]()
)

// GetLocalAddress returns the local address of the UDP socket.
// The socket must be open before calling this function.
func GetLocalAddress() *net.UDPAddr {
	assert.IsNotNil(udpSocket, "UDP socket is not initialized.")
	return udpSocket.LocalAddr().(*net.UDPAddr)
}

// Subscribe registers an observer to receive packets from the UDP socket.
// The observer will receive all packets that are received by the socket.
func Subscribe() chan *Packet {
	return packetObservable.Subscribe()
}

// Open opens a UDP socket on all available IPv4 network interfaces.
// The local port is randomly choosen.
// Returns the local address of the socket and an error if any occurs.
func Open(ipv4addr net.IP) (*net.UDPAddr, error) {
	assert.Assert(udpSocket == nil, "UDP socket is already initialized. Call Close() before calling Open() again.")

	socket, err := net.ListenUDP("udp4", &net.UDPAddr{
		IP:   ipv4addr,
		Port: 0})
	if err != nil {
		return nil, err
	}

	udpSocket = socket

	go readLoop()

	return socket.LocalAddr().(*net.UDPAddr), nil
}

func readLoop() {
	for {
		buffer := make([]byte, common.UDP_BUFFER_SIZE_BYTES)
		n, addr, err := udpSocket.ReadFromUDP(buffer)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				// Socket is closed, exit the loop
				return
			}

			logger.Warnf("Failed to read from UDP socket: %v", err)
			continue
		}

		packetObservable.NotifyObservers(&Packet{addr, buffer[:n]})
	}
}

// SendTo sends a byte array to the specified address.
// Open() must be called before using this function.
func SendTo(addr *net.UDPAddr, data []byte) error {
	assert.IsNotNil(udpSocket, "UDP socket is not initialized.")

	_, err := udpSocket.WriteToUDP(data, addr)
	if err != nil {
		return err
	}

	return nil
}

// Close closes the UDP socket if it's open.
// Observers are not cleared, they will receive packets from future sockets.
func Close() error {
	if udpSocket == nil {
		return nil
	}

	err := udpSocket.Close()
	if err != nil {
		return err
	}

	udpSocket = nil

	return nil
}

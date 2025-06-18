// Package socket manages the UDP socket. The socket can send and receive UDP packets.
// There is only one socket per application. All packets are sent through this socket.
package sock

import (
	"errors"
	"net"
	"net/netip"

	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/util/assert"
	"bjoernblessin.de/chatprotogol/util/logger"
	"bjoernblessin.de/chatprotogol/util/observer"
)

type Socket interface {
	// GetLocalAddress returns the local address of the UDP socket.
	// It errors if the socket is not initialized.
	GetLocalAddress() (netip.AddrPort, error)

	// MustGetLocalAddress returns the local address of the UDP socket.
	// It panics if the socket is not initialized.
	MustGetLocalAddress() netip.AddrPort

	// SendTo sends a byte array to the specified address.
	// Open() must be called before using this function.
	SendTo(addr *net.UDPAddr, data []byte) error

	// Open opens a UDP socket on all available IPv4 network interfaces.
	// The local port is randomly choosen.
	// Returns the local address of the socket and an error if any occurs.
	Open(ipv4addr net.IP) (*net.UDPAddr, error)

	// Close closes the UDP socket if it's open.
	// Packet observers are not cleared, they will receive packets from future sockets.
	Close() error

	// Subscribe registers an observer to receive packets from the UDP socket.
	// The observer will receive all packets that are received by the socket.
	Subscribe() chan *Packet
}

type udpSocket struct {
	udpSocket        *net.UDPConn
	packetObservable *observer.Observable[*Packet]
}

type Packet struct {
	Addr *net.UDPAddr
	Data []byte
}

func NewUDPSocket() *udpSocket {
	return &udpSocket{
		packetObservable: observer.NewObservable[*Packet](common.SOCKET_RECEIVE_BUFFER_SIZE),
	}
}

func (s *udpSocket) GetLocalAddress() (netip.AddrPort, error) {
	if s.udpSocket == nil {
		return netip.AddrPort{}, errors.New("UDP socket is not initialized")
	}
	return s.udpSocket.LocalAddr().(*net.UDPAddr).AddrPort(), nil
}

func (s *udpSocket) MustGetLocalAddress() netip.AddrPort {
	addr, err := s.GetLocalAddress()
	assert.IsNil(err)
	return addr
}

func (s *udpSocket) Subscribe() chan *Packet {
	return s.packetObservable.Subscribe()
}

func (s *udpSocket) Open(ipv4addr net.IP) (*net.UDPAddr, error) {
	assert.Assert(s.udpSocket == nil, "UDP socket is already initialized. Call Close() before calling Open() again.")

	socket, err := net.ListenUDP("udp4", &net.UDPAddr{
		IP:   ipv4addr,
		Port: 0})
	if err != nil {
		return nil, err
	}

	s.udpSocket = socket

	go s.readLoop()

	return socket.LocalAddr().(*net.UDPAddr), nil
}

func (s *udpSocket) readLoop() {
	for {
		buffer := make([]byte, common.UDP_BUFFER_SIZE_BYTES)
		n, addr, err := s.udpSocket.ReadFromUDP(buffer)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				// Socket is closed, exit the loop
				return
			}

			logger.Warnf("Failed to read from UDP socket: %v", err)
			continue
		}

		s.packetObservable.NotifyObservers(&Packet{addr, buffer[:n]})
	}
}

func (s *udpSocket) SendTo(addr *net.UDPAddr, data []byte) error {
	assert.IsNotNil(s.udpSocket, "UDP socket is not initialized.")

	_, err := s.udpSocket.WriteToUDP(data, addr)
	if err != nil {
		return err
	}

	return nil
}

func (s *udpSocket) Close() error {
	if s.udpSocket == nil {
		return nil
	}

	err := s.udpSocket.Close()
	if err != nil {
		return err
	}

	s.udpSocket = nil

	return nil
}

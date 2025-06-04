// Package host manages information about hosts in the network.
package host

import "net"

type Host struct {
	IP             net.IP // The unique identifier of the host
	SequenceNumber uint32 // Sequence number for message ordering
}

func newHost(ip net.IP) *Host {
	return &Host{
		IP: ip,
	}
}

// SendMessage sends a message of the specified type to the host.
func (h *Host) SendMessage() {

}

func GetAvailableHosts() []*Host {
	return []*Host{}
}

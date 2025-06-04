package host

import (
	"net"

	"bjoernblessin.de/chatprotogol/util/assert"
)

type HostManager struct {
	hosts map[string]*Host // Maps host IPs to Host objects
}

func NewHostManager() *HostManager {
	return &HostManager{
		hosts: make(map[string]*Host),
	}
}

// AddHost adds a new host to the list of managed hosts.
// There must not be a host with the same IP address already present.
// The IP address must be a valid IP address.
func (hm *HostManager) AddHost(ip string) {
	var ipaddr = net.ParseIP(ip)
	assert.IsNotNil(ipaddr, "Invalid IP address: %s", ip)

	_, exists := hm.hosts[ip]
	assert.Assert(!exists, "Host with IP %s already exists", ip)

	hm.hosts[ip] = newHost(ipaddr)
}

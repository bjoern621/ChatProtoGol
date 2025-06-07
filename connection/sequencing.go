package connection

import "net/netip"

var SequenceNumbers = make(map[netip.Addr]uint32)

// GetNextSequenceNumber returns the next sequence number for the given address.
func GetNextSequenceNumber(addr netip.Addr) uint32 {
	seqNum, exists := SequenceNumbers[addr]
	if !exists {
		seqNum = 0
	}

	SequenceNumbers[addr] = seqNum + 1

	return seqNum
}

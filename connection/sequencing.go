package connection

import "net/netip"

var sequenceNumbers = make(map[netip.AddrPort]uint32)

// getNextSequenceNumber returns the next sequence number for the given address.
func getNextSequenceNumber(addr netip.AddrPort) [4]byte {
	seqNum, exists := sequenceNumbers[addr]
	if !exists {
		seqNum = 0
	}

	sequenceNumbers[addr] = seqNum + 1

	return [4]byte{
		byte(seqNum >> 24),
		byte(seqNum >> 16),
		byte(seqNum >> 8),
		byte(seqNum),
	}
}

package pkt

// SetChecksum calculates and sets the checksum for a given packet.
// The current checksum field is irrelevant and will be overwritten.
// No more modifications to the packet should be made after setting the checksum.
func SetChecksum(packet *Packet) {
	packet.Header.Checksum = [2]byte{0, 0} // Clear the checksum field before calculation

	checksum := calculateChecksum(packet)

	// Invert the bits to get the checksum
	checksum[0] = ^checksum[0]
	checksum[1] = ^checksum[1]

	packet.Header.Checksum = checksum
}

// calculateChecksum computes the checksum for a given packet.
// It calculates the Checksum using the TCP/IP checksum algorithm,
// which involves summing 16-bit words and folding the result to 16 bits.
func calculateChecksum(packet *Packet) [2]byte {
	data := packet.ToByteArray()

	var sum uint32
	for i := 0; i < len(data); i += 2 {
		if i+1 < len(data) {
			sum += uint32(data[i])<<8 | uint32(data[i+1])
		} else {
			sum += uint32(data[i]) << 8
		}
	}

	// Fold 32-bit sum to 16 bits
	for sum>>16 > 0 { // While loop because we might have overflow after adding
		sum = (sum & 0xFFFF) + (sum >> 16)
	}

	return [2]byte{byte(sum >> 8), byte(sum & 0xFF)}
}

// VerifyChecksum validates the checksum of a packet to ensure data integrity.
// Returns true if the checksums match, false otherwise.
func VerifyChecksum(packet *Packet) bool {
	calculatedChecksum := calculateChecksum(packet)
	return calculatedChecksum == [2]byte{0xff, 0xff}
}

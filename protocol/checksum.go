package protocol

func CalculateChecksum(data []byte) [2]byte {
	var sum uint32
	for i := 0; i < len(data); i += 2 {
		if i+1 < len(data) {
			sum += uint32(data[i])<<8 | uint32(data[i+1])
		} else {
			sum += uint32(data[i]) << 8
		}
	}

	// Fold 32-bit sum to 16 bits
	for sum>>16 > 0 {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}

	return [2]byte{byte(sum >> 8), byte(sum & 0xFF)}
}

func VerifyChecksum(data []byte, checksum [2]byte) bool {
	calculated := CalculateChecksum(data)
	return calculated == checksum
}

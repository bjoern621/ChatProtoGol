package handler

import (
	"errors"
	"net/netip"

	"slices"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/routing"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/sock"
	"bjoernblessin.de/chatprotogol/util/logger"
)

func handleDatabaseDescription(packet *pkt.Packet, router *routing.Router, inSequencing *sequencing.IncomingPktNumHandler, socket sock.Socket) {
	duplicate, dupErr := inSequencing.IsDuplicatePacket(packet)
	if dupErr != nil {
		return
	} else if duplicate {
		_ = connection.SendRoutedAcknowledgment(netip.AddrFrom4(packet.Header.SourceAddr), packet.Header.PktNum)
		return
	}

	logger.Infof("DD RECEIVED %v %d", packet.Header.SourceAddr, packet.Header.PktNum)

	sourceAddr := netip.AddrFrom4(packet.Header.SourceAddr)

	_ = connection.SendRoutedAcknowledgment(sourceAddr, packet.Header.PktNum)

	existingAddresses, err := parseDatabaseDescriptionPayload(packet.Payload)
	if err != nil {
		logger.Warnf("Failed to parse DD payload: %v", err)
		return
	}

	missing := getMissingLSAs(existingAddresses, router)

	for _, missingAddr := range missing {
		lsa, exists := router.GetLSA(missingAddr)
		if !exists {
			continue // LSDB changed between getMissingLSAs() and here (very unlikely)
		}
		connection.FloodLSA(lsa)
	}
}

// func sendLSA(lsaAddr netip.Addr, destinationAddr netip.Addr, router *routing.Router, socket sock.Socket) {
// 	lsa, exists := router.GetLSA(lsaAddr)
// 	assert.Assert(exists, "LSA should exist for the address")

// 	payload := make(pkt.Payload, 0, 8+len(lsa.Neighbors)*4)

// 	localAddressBytes := socket.MustGetLocalAddress().Addr().As4()
// 	payload = append(payload, localAddressBytes[:]...)

// 	binary.BigEndian.PutUint32(payload[4:8], lsa.SeqNum)

// 	for _, neighborAddr := range lsa.Neighbors {
// 		addrBytes := neighborAddr.As4()
// 		payload = append(payload, addrBytes[:]...)
// 	}

// 	packet := connection.BuildSequencedPacket(pkt.MsgTypeLSA, true, payload, destinationAddr)

// 	err := connection.SendReliableRoutedPacket(packet)
// 	if err != nil {
// 		logger.Warnf("Failed to send LSA for %s: %v", destinationAddr, err)
// 	}
// }

// getMissingLSAs compares the existing entries with the LSAs in the LSDB.
// It returns a slice of addresses that are in the local LSDB but not in the existing entries.
// This is used to determine which LSAs need to be sent to the peer.
func getMissingLSAs(existingEntries []netip.Addr, router *routing.Router) []netip.Addr {
	missingEntries := make([]netip.Addr, 0)

	for _, addr := range router.GetAvailableLSAs() {
		if !slices.Contains(existingEntries, addr) {
			missingEntries = append(missingEntries, addr)
		}
	}

	return missingEntries
}

func parseDatabaseDescriptionPayload(payload pkt.Payload) ([]netip.Addr, error) {
	const bytesPerAddr = 4

	if len(payload)%bytesPerAddr != 0 {
		return nil, errors.New("invalid payload length for DD packet")
	}

	entries := make([]netip.Addr, 0, len(payload)/bytesPerAddr)

	for i := 0; i < len(payload); i += bytesPerAddr {
		addrBytes := payload[i:(i + bytesPerAddr)]

		addr, ok := netip.AddrFromSlice(addrBytes)
		if !ok || !addr.Is4() {
			return nil, errors.New("invalid IPv4 address in DD packet")
		}

		entries = append(entries, addr)
	}

	return entries, nil
}

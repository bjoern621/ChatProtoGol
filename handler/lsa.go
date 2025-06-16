package handler

import (
	"encoding/binary"
	"errors"
	"net/netip"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/routing"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/util/logger"
)

func handleLSA(packet *pkt.Packet, router *routing.Router, inSequencing *sequencing.IncomingPktNumHandler) {
	duplicate, dupErr := inSequencing.IsDuplicatePacket(packet)
	if dupErr != nil {
		return
	} else if duplicate {
		_ = connection.SendRoutedAcknowledgment(netip.AddrFrom4(packet.Header.SourceAddr), packet.Header.PktNum)
		return
	}

	logger.Infof("LSA RECEIVED %v %d", packet.Header.SourceAddr, packet.Header.PktNum)

	sourceAddr := netip.AddrFrom4(packet.Header.SourceAddr)

	_ = connection.SendRoutedAcknowledgment(sourceAddr, packet.Header.PktNum)

	srcAddr, seqNum, neighborAddresses, err := parseLSAPayload(packet.Payload)
	if err != nil {
		logger.Warnf("Failed to parse LSA payload: %v", err)
		return
	}

	existingLSA, exists := router.GetLSA(sourceAddr)
	if exists && existingLSA.SeqNum >= seqNum {
		logger.Infof("Received LSA of %v(seq num: %v) from %v(pkt num: %v), but already have seqnum %d", srcAddr, seqNum, sourceAddr, packet.Header.PktNum, existingLSA.SeqNum)
		return
	}

	router.AddLSA(srcAddr, seqNum, neighborAddresses)

	updatedLSA, exists := router.GetLSA(srcAddr)
	if !exists {
		logger.Warnf("LSA for %v not found after adding it to the LSDB", srcAddr)
		return
	}

	connection.FloodLSA(updatedLSA) // TODO dont send to sender
}

func parseLSAPayload(payload pkt.Payload) (srcAddr netip.Addr, seqNum uint32, neighborAddresses []netip.Addr, err error) {
	if 8+len(payload)%4 != 8 {
		return netip.Addr{}, 0, nil, errors.New("invalid payload length for LSA packet")
	}

	srcAddr, ok := netip.AddrFromSlice(payload[:4])
	if !ok || !srcAddr.Is4() {
		return netip.Addr{}, 0, nil, errors.New("invalid source address in LSA packet")
	}

	seqNum = binary.BigEndian.Uint32(payload[4:8])

	neighborAddresses = make([]netip.Addr, 0, len(payload[8:])/4)

	for i := 8; i < len(payload); i += 4 {
		addrBytes := payload[i:(i + 4)]

		addr, ok := netip.AddrFromSlice(addrBytes)
		if !ok || !addr.Is4() {
			return netip.Addr{}, 0, nil, errors.New("invalid neighbor IPv4 address in LSA packet")
		}

		neighborAddresses = append(neighborAddresses, addr)
	}

	return
}

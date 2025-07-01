package handler

import (
	"encoding/binary"
	"errors"
	"net/netip"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/routing"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/sock"
	"bjoernblessin.de/chatprotogol/util/logger"
)

func handleLSA(packet *pkt.Packet, router *routing.Router, inSequencing *sequencing.IncomingPktNumHandler, srcAddrPort netip.AddrPort, socket sock.Socket) {
	duplicate, dupErr := inSequencing.IsDuplicatePacket(packet)
	if dupErr != nil {
		logger.Warnf(dupErr.Error())
		return
	} else if duplicate {
		_ = connection.SendAcknowledgmentTo(srcAddrPort, packet.Header.PktNum)
		return
	}

	logger.Debugf("LSA RECEIVED %v %d", packet.Header.SourceAddr, packet.Header.PktNum)

	srcAddr := netip.AddrFrom4(packet.Header.SourceAddr)
	if srcAddr != srcAddrPort.Addr() {
		logger.Warnf("Malformed LSA packet: source address %v does not match sender %v", srcAddr, srcAddrPort)
		return
	}

	destAddr := netip.AddrFrom4(packet.Header.DestAddr)
	localAddr := socket.MustGetLocalAddress().Addr()
	if destAddr != localAddr {
		logger.Warnf("Malformed LSA packet: destination address %v does not match local address %v", destAddr, localAddr)
		return
	}

	lsaOwnerAddr, seqNum, neighborAddresses, err := parseLSAPayload(packet.Payload)
	if err != nil {
		logger.Warnf("Failed to parse LSA payload: %v", err)
		return
	}

	// Valid packet

	_ = connection.SendAcknowledgmentTo(srcAddrPort, packet.Header.PktNum)

	logger.Infof("LSA of %v with seqnum %d, neighbors: %v", lsaOwnerAddr, seqNum, neighborAddresses)

	existingLSA, exists := router.GetLSA(lsaOwnerAddr)
	if exists && existingLSA.SeqNum >= seqNum {
		logger.Debugf("Received LSA of %v(seqnum: %v) from %v(pkt num: %v), but already have seqnum %d", lsaOwnerAddr, seqNum, srcAddr, packet.Header.PktNum, existingLSA.SeqNum)
		return
	}

	notRoutableHosts := router.UpdateLSA(lsaOwnerAddr, seqNum, neighborAddresses)
	connection.ClearUnreachableHosts(notRoutableHosts)

	updatedLSA, exists := router.GetLSA(lsaOwnerAddr)
	if !exists {
		logger.Warnf("LSA for %v not found after adding it to the LSDB", lsaOwnerAddr)
		return
	}

	connection.FloodLSA(lsaOwnerAddr, updatedLSA, srcAddr)
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

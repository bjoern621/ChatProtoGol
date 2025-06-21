package reconstruction

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"

	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/util/assert"
)

// OnDiskReconstructor is responsible for reconstructing file transfer packets.
// It stores state needed to reconstruct one open file transfer sequence at a time.
type OnDiskReconstructor struct {
	packetBuffer           map[int64]pkt.Payload
	lowestPktNum           int64
	highestWrittenPktNum   int64
	highestUnwrittenPktNum int64
	file                   *os.File
	inSequencing           *sequencing.IncomingPktNumHandler
	peerAddr               netip.Addr
}

func NewOnDiskReconstructor(inSeq *sequencing.IncomingPktNumHandler, peerAddr netip.Addr) *OnDiskReconstructor {
	return &OnDiskReconstructor{
		packetBuffer:           make(map[int64]pkt.Payload),
		lowestPktNum:           -1,
		highestWrittenPktNum:   -1,
		highestUnwrittenPktNum: -1,
		inSequencing:           inSeq,
		peerAddr:               peerAddr,
	}
}

// HandleIncomingFilePacket processes an incoming file transfer packet.
func (r *OnDiskReconstructor) HandleIncomingFilePacket(packet *pkt.Packet) error {
	pktNum := int64(binary.BigEndian.Uint32(packet.Header.PktNum[:]))

	r.packetBuffer[pktNum] = packet.Payload

	if r.file == nil {
		fmt.Printf("Creating new file for reconstruction for %v\n", r.peerAddr)
		file, err := os.CreateTemp("", "recon_")
		if err != nil {
			return errors.New("failed to create file for file reconstruction")
		}
		r.file = file
	}

	if r.lowestPktNum < 0 {
		// This is the first packet, initialize lowestPktNum
		r.lowestPktNum = pktNum
		r.highestWrittenPktNum = pktNum

		return nil
	}

	if pktNum < r.lowestPktNum {
		r.lowestPktNum = pktNum
	}

	if pktNum > r.highestUnwrittenPktNum {
		r.highestUnwrittenPktNum = pktNum
	}

	r.flushContiguousPayloads()

	return nil
}

// flushBuffer writes buffered packets to disk and clears the buffer.
func (r *OnDiskReconstructor) flushContiguousPayloads() {
	for i := r.highestWrittenPktNum + 1; i <= r.highestUnwrittenPktNum; i++ {
		payload, ok := r.packetBuffer[i]
		if i > r.inSequencing.GetHighestContiguousSeqNum(r.peerAddr) {
			return // Can't write this packet yet, it is not contiguous
		}

		if !ok {
			continue // Skip if no payload for this packet number
		}

		_, err := r.file.Write(payload)
		if err != nil {
			assert.IsNil(err, "failed to write payload to file in flushContiguousPayloads")
			return
		}
		delete(r.packetBuffer, i)
		r.highestWrittenPktNum = i
	}
}

func (r *OnDiskReconstructor) flushRemainingPayloads() {
	for i := r.highestWrittenPktNum + 1; i <= r.highestUnwrittenPktNum; i++ {
		payload, ok := r.packetBuffer[i]
		if !ok {
			continue // Skip if no payload for this packet number
		}

		_, err := r.file.Write(payload)
		if err != nil {
			fmt.Printf("awdwadwad")
			assert.IsNil(err, "failed to write remaining payload to file in flushRemainingPayloads")
			return
		}
		delete(r.packetBuffer, i)
	}
}

// FinishFilePacketSequence completes the current packet sequence for a specific source address.
// It returns the file path of the reconstructed file.
func (r *OnDiskReconstructor) FinishFilePacketSequence() (string, error) {
	r.flushRemainingPayloads()

	err := r.file.Close()
	assert.IsNil(err, "failed to close file in FinishFilePacketSequence")

	metadataPayload, exists := r.packetBuffer[r.lowestPktNum]
	assert.Assert(exists, "lowestPktNum not found in packetBuffer")

	const FILE_NAME_SIZE_BYTES = 1024
	n := min(len(metadataPayload), FILE_NAME_SIZE_BYTES)
	fileName := string(metadataPayload[:n])

	dir := common.RECEIVED_FILES_DIR
	err = os.MkdirAll(dir, 0700) // owner read/write/execute, group and others no permissions
	if err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	err = os.Rename(r.file.Name(), filepath.Join(dir, fileName))
	if err != nil {
		return "", fmt.Errorf("failed to rename file: %w", err)
	}

	return filepath.Join(dir, fileName), nil
}

// GetHighestPktNum returns the highest packet number that has been processed by this reconstructor.
func (r *OnDiskReconstructor) GetHighestPktNum() (uint32, error) {
	if r.highestUnwrittenPktNum < 0 {
		return 0, errors.New("no packets buffered")
	}

	return uint32(r.highestUnwrittenPktNum), nil
}

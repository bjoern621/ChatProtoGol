package reconstruction

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"sync"

	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/util/assert"
)

// OnDiskReconstructor is responsible for reconstructing file transfer packets.
// It stores state needed to reconstruct one open file transfer sequence at a time.
// The OnDiskReconstructor is thread-safe and can be used concurrently.
type OnDiskReconstructor struct {
	packetBuffer           map[int64]pkt.Payload
	lowestPktNum           int64
	highestWrittenPktNum   int64
	highestUnwrittenPktNum int64
	file                   *os.File
	inSequencing           *sequencing.IncomingPktNumHandler
	peerAddr               netip.Addr
	mu                     sync.Mutex // Mutex to protect concurrent access to the (whole) reconstructor
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
	r.mu.Lock()
	defer r.mu.Unlock()

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
		r.highestWrittenPktNum = pktNum // If we receive a packet with a lower number than the lowest, we know that we have not written any packets yet, so we can reset the highestWrittenPktNum
	}

	if pktNum > r.highestUnwrittenPktNum {
		r.highestUnwrittenPktNum = pktNum
	}

	r.flushContiguousPayloads()

	return nil
}

// flushBuffer writes buffered packets to disk and clears the buffer.
func (r *OnDiskReconstructor) flushContiguousPayloads() {
	// highestContiguousPktNum := r.inSequencing.GetHighestContiguousSeqNum(r.peerAddr)
	// TODO ^^^
	// TODO bug out of order received, GetHighestContiguousSeqNum() already updated, but HandleIncomingFilePacket() not called yet with the new packet
	// TODO meaning that GetHighestContiguousSeqNum() is e.g. N, we try to write N-2 til N, we write N-2 and N-1, but N is not written (it is skipped) because we dont have it in the buffer yet
	// TODO we should keep our on GetHighestContiguousSeqNum() and dont rely on sequencing
	// TODO maybe a method like HandleIncomingNonFilePacket()

	for i := r.highestWrittenPktNum + 1; i <= r.highestUnwrittenPktNum; i++ {
		// if i > highestContiguousPktNum {
		// 	return // Can't write this packet yet, it is not contiguous
		// }

		payload, ok := r.packetBuffer[i]
		if !ok {
			return
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
			continue // Skip if no payload for this packet number; this means that the packet with packet number i is not a file transfer packet
		}

		_, err := r.file.Write(payload)
		if err != nil {
			assert.IsNil(err, "failed to write remaining payload to file in flushRemainingPayloads")
			return
		}
		delete(r.packetBuffer, i)
	}
}

// FinishFilePacketSequence completes the current packet sequence for a specific source address.
// It returns the file path of the reconstructed file.
func (r *OnDiskReconstructor) FinishFilePacketSequence() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

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
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.highestUnwrittenPktNum < 0 {
		return 0, errors.New("no packets buffered")
	}

	return uint32(r.highestUnwrittenPktNum), nil
}

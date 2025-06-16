package reconstruction

import (
	"net/netip"

	"bjoernblessin.de/chatprotogol/sequencing"
)

type PktSequenceReconstructor struct {
	sequencing    *sequencing.IncomingPktNumHandler
	payloadBuffer map[netip.Addr]*buffer // Maps host addresses to buffer information
}

func NewPktSequenceReconstructor(sequencing *sequencing.IncomingPktNumHandler) *PktSequenceReconstructor {
	return &PktSequenceReconstructor{
		sequencing:    sequencing,
		payloadBuffer: make(map[netip.Addr]*buffer),
	}
}

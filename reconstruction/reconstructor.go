package reconstruction

import "bjoernblessin.de/chatprotogol/sequencing"

type PktSequenceReconstructor struct {
	sequencing *sequencing.IncomingPktNumHandler
}

func NewPktSequenceReconstructor(sequencing *sequencing.IncomingPktNumHandler) *PktSequenceReconstructor {
	return &PktSequenceReconstructor{
		sequencing: sequencing,
	}
}

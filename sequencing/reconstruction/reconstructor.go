package reconstruction

type Reconstructor interface {
	// GetHighestPktNum returns the highest packet number that has been processed by this reconstructor.
	GetHighestPktNum() (uint32, error)
}

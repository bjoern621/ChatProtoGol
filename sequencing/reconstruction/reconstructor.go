package reconstruction

import (
	"fmt"
	"net/netip"
	"sync"

	"bjoernblessin.de/chatprotogol/util/logger"
)

type Reconstructor interface {
	// GetHighestPktNum returns the highest packet number that has been processed by this reconstructor.
	GetHighestPktNum() (uint32, error)
	// ClearState clears the internal state of the reconstructor. The reconstructor cannot be reused after clearing the state.
	ClearState() error
}

var (
	fileReconstructors      = make(map[netip.Addr]*OnDiskReconstructor)
	fileReconstructorsMutex sync.Mutex
)

var (
	msgReconstructors      = make(map[netip.Addr]*InMemoryReconstructor)
	msgReconstructorsMutex sync.Mutex
)

func GetFileReconstructor(addr netip.Addr) *OnDiskReconstructor {
	fileReconstructorsMutex.Lock()
	defer fileReconstructorsMutex.Unlock()

	reconstructor, exists := fileReconstructors[addr]
	if !exists {
		fmt.Print("Creating new file reconstructor for ", addr, "\n")
		reconstructor = NewOnDiskReconstructor(addr)
		fileReconstructors[addr] = reconstructor
	}

	fileReconstructors[addr] = reconstructor
	return reconstructor
}

func GetMsgReconstructor(addr netip.Addr) *InMemoryReconstructor {
	msgReconstructorsMutex.Lock()
	defer msgReconstructorsMutex.Unlock()

	reconstructor, exists := msgReconstructors[addr]
	if !exists {
		reconstructor = NewInMemoryReconstructor()
		msgReconstructors[addr] = reconstructor
	}

	return reconstructor
}

func ClearReconstructors(addr netip.Addr) {
	fileReconstructorsMutex.Lock()
	defer fileReconstructorsMutex.Unlock()
	msgReconstructorsMutex.Lock()
	defer msgReconstructorsMutex.Unlock()

	if reconstructor, exists := fileReconstructors[addr]; exists {
		reconstructor.ClearState()
		delete(fileReconstructors, addr)
	}

	if reconstructor, exists := msgReconstructors[addr]; exists {
		reconstructor.ClearState()
		delete(msgReconstructors, addr)
	}

	logger.Debugf("Cleared all reconstructor states for %v", addr)
}

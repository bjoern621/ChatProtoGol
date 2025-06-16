package routing

import (
	"net/netip"
	"sync"

	"bjoernblessin.de/chatprotogol/reconstruction"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/sock"
)

type Router struct {
	lsdb          map[netip.Addr]LSAEntry // Link State Database (LSDB) that holds the Link State Advertisements (LSAs) of every host (including the local LSA)
	socket        sock.Socket
	neighborTable map[netip.Addr]NeighborEntry
	routingTable  map[netip.Addr]netip.AddrPort // Maps destination IP addresses to the next hop they should use
	mu            sync.Mutex                    // Protects access to the router's state, including the LSDB, neighbor table, and routing table
	inSequencing  *sequencing.IncomingPktNumHandler
	outSequencing *sequencing.OutgoingPktNumHandler
	reconstructor *reconstruction.PktSequenceReconstructor
}

func NewRouter(socket sock.Socket, inSequencing *sequencing.IncomingPktNumHandler, outSequencing *sequencing.OutgoingPktNumHandler, reconstructor *reconstruction.PktSequenceReconstructor) *Router {
	return &Router{
		lsdb:          make(map[netip.Addr]LSAEntry),
		socket:        socket,
		neighborTable: make(map[netip.Addr]NeighborEntry),
		routingTable:  make(map[netip.Addr]netip.AddrPort),
		inSequencing:  inSequencing,
		outSequencing: outSequencing,
		reconstructor: reconstructor,
	}
}

// AddNeighbor adds a new neighbor to the router.
// It adds the neighbor to the neighbor table, recalculates the local LSA, and builds the routing table.
// Can be called concurrently.
func (r *Router) AddNeighbor(nextHop netip.AddrPort) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.addNeighbor(nextHop)
	r.recalculateLocalLSA()
	_ = r.buildRoutingTable() // there should be no unreachable hosts
}

// RemoveNeighbor removes a neighbor from the router.
// It removes the neighbor from the neighbor table, recalculates the local LSA, and builds the routing table.
// May delete hosts that are no longer reachable.
// Can be called concurrently.
func (r *Router) RemoveNeighbor(addr netip.Addr) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.removeNeighbor(addr)
	r.recalculateLocalLSA()
	unreachableHosts := r.buildRoutingTable()
	r.clearUnreachableHosts(unreachableHosts)
}

// UpdateLSA adds a new LSA to the router.
// It updates the LSA in the LSDB and builds the routing table.
// May delete hosts that are no longer reachable.
// Can be called concurrently.
func (r *Router) UpdateLSA(srcAddr netip.Addr, seqNum uint32, neighborAddresses []netip.Addr) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.updateLSA(srcAddr, seqNum, neighborAddresses)
	unreachableHosts := r.buildRoutingTable()
	r.clearUnreachableHosts(unreachableHosts)
}

// clearUnreachableHosts clears state for hosts that are no longer reachable.
// This includes removing their LSAs from the LSDB, their sequencing state and their payload buffer in the reconstruction package.
// May be called with the zero list in which case it does nothing.
func (r *Router) clearUnreachableHosts(unreachableHosts []netip.Addr) {
	for _, addr := range unreachableHosts {
		delete(r.lsdb, addr)
		r.inSequencing.ClearIncomingPacketNumbers(addr)
		r.outSequencing.ClearPacketNumbers(addr)
		r.reconstructor.ClearPayloadBuffer(addr)
	}
}

// TODO
type Rtable struct {
	Entries map[netip.Addr]ee
}

type ee struct {
	NextHop  netip.AddrPort
	HopCount int
}

func IsNeighbor2(addr netip.Addr) (bool, netip.AddrPort) {
	// This function should check if the address is a neighbor and return the corresponding AddrPort
	// For now, we return false and an empty AddrPort
	return false, netip.AddrPort{}
}

func ParseRoutingTableFromPayload(payload []byte, src netip.AddrPort) (*Rtable, error) {
	// This function should parse the routing table from the payload
	// For now, we return an empty Rtable and nil error
	return &Rtable{Entries: make(map[netip.Addr]ee)}, nil
}

func UpdateRoutingTable(routingTable *Rtable, src netip.AddrPort) bool {
	// This function should update the routing table with the new entries
	// For now, we do nothing
	return false
}

func GetRoutingTableEntries() map[netip.Addr]netip.AddrPort {
	// This function should return the routing table entries
	return make(map[netip.Addr]netip.AddrPort)
}

func RemoveRoutingEntriesWithNextHop(addrPort netip.AddrPort) {
	// This function should remove all routing entries that use the specified next hop
	// For now, we do nothing
}

func RemoveRoutingEntry(addr netip.Addr) {
	// This function should remove the routing entry for the specified address
	// For now, we do nothing
}

func GetNextHop2(addr netip.Addr) (netip.AddrPort, bool) {
	// This function should return the next hop for the specified address
	// For now, we return an empty AddrPort and false
	return netip.AddrPort{}, false
}

func FormatRoutingTableForPayload() []byte {
	// This function should format the routing table for payload
	// For now, we return an empty byte slice
	return []byte{}
}

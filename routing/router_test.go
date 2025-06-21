package routing

import (
	"maps"
	"net/netip"
	"testing"
)

func TestGetUnreachableHosts(t *testing.T) {
	n1 := netip.MustParseAddr("10.0.0.1")
	n2 := netip.MustParseAddr("10.0.0.2")
	n3 := netip.MustParseAddr("10.0.0.3")
	n4 := netip.MustParseAddr("10.0.0.4")
	n5 := netip.MustParseAddr("10.0.0.5")
	n6 := netip.MustParseAddr("10.0.0.6")

	tests := []struct {
		name            string
		lsdb            map[netip.Addr]LSAEntry
		routingTable    map[netip.Addr]netip.AddrPort
		notRoutable     []netip.Addr
		lsaOwner        netip.Addr
		oldLSA          LSAEntry
		expectedUnreach []netip.Addr
	}{
		{
			name: "disconnected networks n1 and n2 / n3 and n4",
			lsdb: map[netip.Addr]LSAEntry{
				n1: {Neighbors: []netip.Addr{n2}},
				n2: {Neighbors: []netip.Addr{n1}}, // n2 drops n3
				n3: {Neighbors: []netip.Addr{n2, n4}},
				n4: {Neighbors: []netip.Addr{n3}},
			},
			routingTable: map[netip.Addr]netip.AddrPort{
				n2: {},
			},
			notRoutable:     []netip.Addr{n3, n4},
			lsaOwner:        n2,
			oldLSA:          LSAEntry{Neighbors: []netip.Addr{n1, n3}},
			expectedUnreach: []netip.Addr{n3, n4},
		},
		{
			name: "no neighbor removed",
			lsdb: map[netip.Addr]LSAEntry{
				n1: {Neighbors: []netip.Addr{n2}},
				n2: {Neighbors: []netip.Addr{n1, n3}},
				n3: {Neighbors: []netip.Addr{n2, n4}},
				n4: {Neighbors: []netip.Addr{n3}},
			},
			routingTable: map[netip.Addr]netip.AddrPort{
				n2: {}, n3: {}, n4: {},
			},
			notRoutable:     []netip.Addr{},
			lsaOwner:        n2,
			oldLSA:          LSAEntry{Neighbors: []netip.Addr{n1, n3}},
			expectedUnreach: nil,
		},
		{
			name: "removed neighbor still routable via loop",
			lsdb: map[netip.Addr]LSAEntry{
				// n1 <-> n2 <-> n3 <-> n4 <-> n1
				n1: {Neighbors: []netip.Addr{n2, n4}},
				n2: {Neighbors: []netip.Addr{n1}}, // n2 drops n3
				n3: {Neighbors: []netip.Addr{n2, n4}},
				n4: {Neighbors: []netip.Addr{n3, n1}},
			},
			routingTable: map[netip.Addr]netip.AddrPort{
				n2: {}, n3: {}, n4: {},
			},
			notRoutable:     []netip.Addr{},
			lsaOwner:        n2,
			oldLSA:          LSAEntry{Neighbors: []netip.Addr{n1, n3}},
			expectedUnreach: []netip.Addr{},
		},
		{
			name: "not routable hosts but still considered reachable",
			lsdb: map[netip.Addr]LSAEntry{
				// n1 <-> n2 <-> n3
				//  ^-> n4 <-> n5 <-> n6
				// n1 and n2 just connected, n1 received LSA from n3 but not yet from n2
				// now n4 triggers a LSA update, because it lost connection to n5
				n1: {Neighbors: []netip.Addr{n4}},
				n3: {Neighbors: []netip.Addr{n2}},
				n4: {Neighbors: []netip.Addr{n1}},
				n5: {Neighbors: []netip.Addr{n4, n6}},
				n6: {Neighbors: []netip.Addr{n5}},
			},
			routingTable: map[netip.Addr]netip.AddrPort{
				n4: {},
			},
			notRoutable:     []netip.Addr{n5, n6, n3},
			lsaOwner:        n4,
			oldLSA:          LSAEntry{Neighbors: []netip.Addr{n1, n5}},
			expectedUnreach: []netip.Addr{n5, n6},
		},
		{
			name: "removed neighbor still routable through another route",
			lsdb: map[netip.Addr]LSAEntry{
				// n1 <-> n2 <-> n3 <-> n4  <-v
				//         ^-> n5 <-> n6    <-^
				n1: {Neighbors: []netip.Addr{n2, n4}},
				n2: {Neighbors: []netip.Addr{n1}}, // n2 drops n3
				n3: {Neighbors: []netip.Addr{n2, n4}},
				n4: {Neighbors: []netip.Addr{n3, n6}},
				n5: {Neighbors: []netip.Addr{n2, n6}},
				n6: {Neighbors: []netip.Addr{n4, n5}},
			},
			routingTable: map[netip.Addr]netip.AddrPort{
				n2: {}, n3: {}, n4: {}, n5: {}, n6: {},
			},
			notRoutable:     []netip.Addr{},
			lsaOwner:        n2,
			oldLSA:          LSAEntry{Neighbors: []netip.Addr{n1, n3}},
			expectedUnreach: []netip.Addr{},
		},
		{
			name: "my removed neighbor still routable",
			lsdb: map[netip.Addr]LSAEntry{
				// n4 <-> n2 <-> n1 <-> n5
				//		   ^------------^
				n1: {Neighbors: []netip.Addr{n5}},     // already updated local LSA
				n2: {Neighbors: []netip.Addr{n4, n5}}, // n2 drops n1
				n4: {Neighbors: []netip.Addr{n2}},
				n5: {Neighbors: []netip.Addr{n2, n1}},
			},
			routingTable: map[netip.Addr]netip.AddrPort{
				n4: {}, n2: {}, n5: {},
			},
			notRoutable:     []netip.Addr{},
			lsaOwner:        n2,
			oldLSA:          LSAEntry{Neighbors: []netip.Addr{n1, n4, n5}},
			expectedUnreach: []netip.Addr{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Router{
				lsdb:         make(map[netip.Addr]LSAEntry),
				routingTable: make(map[netip.Addr]netip.AddrPort),
				socket:       &mockSocket{},
			}
			maps.Copy(r.lsdb, tt.lsdb)
			maps.Copy(r.routingTable, tt.routingTable)

			got := r.getUnreachableHosts(tt.notRoutable, tt.lsaOwner, tt.oldLSA)
			if !DeepEqualUnordered(got, tt.expectedUnreach) {
				t.Errorf("got %v, want %v", got, tt.expectedUnreach)
			}
		})
	}
}

// DeepEqualUnordered compares two slices ignoring order
func DeepEqualUnordered(a, b []netip.Addr) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[netip.Addr]int)
	for _, v := range a {
		m[v]++
	}
	for _, v := range b {
		if m[v] == 0 {
			return false
		}
		m[v]--
	}
	return true
}

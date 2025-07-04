package routing

import (
	"net"
	"net/netip"
	"slices"
	"testing"

	"fmt"

	"bjoernblessin.de/chatprotogol/sock"
)

type mockSocket struct{}

const LOCAL_ADDR = "10.0.0.1"
const LOCAL_PORT = 1234

func (m *mockSocket) GetLocalAddress() (netip.AddrPort, error) {
	return m.MustGetLocalAddress(), nil
}

func (m *mockSocket) MustGetLocalAddress() netip.AddrPort {
	return netip.MustParseAddrPort(LOCAL_ADDR + ":" + fmt.Sprint(LOCAL_PORT))
}

func (m *mockSocket) Close() error {
	return nil
}

func (m *mockSocket) SendTo(addr *net.UDPAddr, data []byte) error {
	return nil
}

func (m *mockSocket) Open(ipv4addr net.IP) (*net.UDPAddr, error) {
	return &net.UDPAddr{
		IP:   ipv4addr,
		Port: 0,
	}, nil
}

func (m *mockSocket) Subscribe() chan *sock.Packet {
	return nil
}

// Helper function to compare two maps
func mapsEqual(m1, m2 map[netip.Addr]netip.AddrPort) bool {
	if len(m1) != len(m2) {
		return false
	}
	for k, v1 := range m1 {
		v2, ok := m2[k]
		if !ok || v1 != v2 {
			return false
		}
	}
	return true
}

func TestBuildRoutingTable(t *testing.T) {
	tests := []struct {
		name                string
		lsdb                map[netip.Addr]LSAEntry
		neighborTable       map[netip.Addr]NeighborEntry
		expected            map[netip.Addr]netip.AddrPort
		expectedUnreachable []netip.Addr
	}{
		{
			name: "Only local LSA", // (10.0.0.1)
			lsdb: map[netip.Addr]LSAEntry{
				netip.MustParseAddr(LOCAL_ADDR): {},
			},
			neighborTable:       map[netip.Addr]NeighborEntry{},
			expected:            map[netip.Addr]netip.AddrPort{},
			expectedUnreachable: nil,
		},
		{
			name: "Only local LSA but with Neighbors", // (10.0.0.2) <-> (10.0.0.1) <-> (10.0.0.3)
			lsdb: map[netip.Addr]LSAEntry{
				netip.MustParseAddr(LOCAL_ADDR): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr("10.0.0.2"),
						netip.MustParseAddr("10.0.0.3"),
					},
				},
			},
			neighborTable: map[netip.Addr]NeighborEntry{
				netip.MustParseAddr("10.0.0.2"): {
					NextHop: netip.MustParseAddrPort("10.0.0.2:20000"),
				},
				netip.MustParseAddr("10.0.0.3"): {
					NextHop: netip.MustParseAddrPort("10.0.0.3:30000"),
				},
			},
			expected: map[netip.Addr]netip.AddrPort{},
		},
		{
			name: "Single neighbor", // (10.0.0.1) <-> (10.0.0.2)
			lsdb: map[netip.Addr]LSAEntry{
				netip.MustParseAddr(LOCAL_ADDR): {
					Neighbors: []netip.Addr{netip.MustParseAddr("10.0.0.2")},
				},
				netip.MustParseAddr("10.0.0.2"): {
					Neighbors: []netip.Addr{netip.MustParseAddr("10.0.0.1")},
				},
			},
			neighborTable: map[netip.Addr]NeighborEntry{
				netip.MustParseAddr("10.0.0.2"): {
					NextHop: netip.MustParseAddrPort("10.0.0.2:123"),
				},
			},
			expected: map[netip.Addr]netip.AddrPort{
				netip.MustParseAddr("10.0.0.2"): netip.MustParseAddrPort("10.0.0.2:123"),
			},
		},
		{
			// (10.0.0.2) <-> (10.0.0.1) <-> (10.0.0.3) <-> (10.0.0.5)
			//                     ^-> (10.0.0.4)
			name: "Multiple neighbors",
			lsdb: map[netip.Addr]LSAEntry{
				netip.MustParseAddr(LOCAL_ADDR): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr("10.0.0.2"),
						netip.MustParseAddr("10.0.0.4"),
						netip.MustParseAddr("10.0.0.3"),
					},
				},
				netip.MustParseAddr("10.0.0.2"): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr("10.0.0.1"),
					},
				},
				netip.MustParseAddr("10.0.0.3"): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr("10.0.0.1"),
						netip.MustParseAddr("10.0.0.5"),
					},
				},
				netip.MustParseAddr("10.0.0.4"): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr("10.0.0.1"),
					},
				},
				netip.MustParseAddr("10.0.0.5"): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr("10.0.0.3"),
					},
				},
			},
			neighborTable: map[netip.Addr]NeighborEntry{
				netip.MustParseAddr("10.0.0.2"): {
					NextHop: netip.MustParseAddrPort("10.0.0.2:20000"),
				},
				netip.MustParseAddr("10.0.0.3"): {
					NextHop: netip.MustParseAddrPort("10.0.0.3:30000"),
				},
				netip.MustParseAddr("10.0.0.4"): {
					NextHop: netip.MustParseAddrPort("10.0.0.4:40000"),
				},
			},
			expected: map[netip.Addr]netip.AddrPort{
				netip.MustParseAddr("10.0.0.2"): netip.MustParseAddrPort("10.0.0.2:20000"),
				netip.MustParseAddr("10.0.0.3"): netip.MustParseAddrPort("10.0.0.3:30000"),
				netip.MustParseAddr("10.0.0.4"): netip.MustParseAddrPort("10.0.0.4:40000"),
				netip.MustParseAddr("10.0.0.5"): netip.MustParseAddrPort("10.0.0.3:30000"),
			},
		},
		{
			//                     v-----------------------------v
			// (10.0.0.1) <-> (10.0.0.2) <-> (10.0.0.3) <-> (10.0.0.4) <-> (10.0.0.5) <-> (10.0.0.6) <-> (10.0.0.1)
			name: "Loop",
			lsdb: map[netip.Addr]LSAEntry{
				netip.MustParseAddr(LOCAL_ADDR): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr("10.0.0.2"),
						netip.MustParseAddr("10.0.0.6"),
					},
				},
				netip.MustParseAddr("10.0.0.2"): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr("10.0.0.1"),
						netip.MustParseAddr("10.0.0.3"),
						netip.MustParseAddr("10.0.0.4"),
					},
				},
				netip.MustParseAddr("10.0.0.3"): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr("10.0.0.2"),
						netip.MustParseAddr("10.0.0.4"),
					},
				},
				netip.MustParseAddr("10.0.0.4"): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr("10.0.0.2"),
						netip.MustParseAddr("10.0.0.3"),
						netip.MustParseAddr("10.0.0.5"),
					},
				},
				netip.MustParseAddr("10.0.0.5"): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr("10.0.0.4"),
						netip.MustParseAddr("10.0.0.6"),
					},
				},
				netip.MustParseAddr("10.0.0.6"): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr("10.0.0.5"),
						netip.MustParseAddr("10.0.0.1"),
					},
				},
			},
			neighborTable: map[netip.Addr]NeighborEntry{
				netip.MustParseAddr("10.0.0.2"): {
					NextHop: netip.MustParseAddrPort("10.0.0.2:20000"),
				},
				netip.MustParseAddr("10.0.0.6"): {
					NextHop: netip.MustParseAddrPort("10.0.0.6:60000"),
				},
			},
			expected: map[netip.Addr]netip.AddrPort{
				// Direct neighbors
				netip.MustParseAddr("10.0.0.2"): netip.MustParseAddrPort("10.0.0.2:20000"),
				netip.MustParseAddr("10.0.0.6"): netip.MustParseAddrPort("10.0.0.6:60000"),

				// Multi-hop destinations - shortest paths
				netip.MustParseAddr("10.0.0.3"): netip.MustParseAddrPort("10.0.0.2:20000"),
				netip.MustParseAddr("10.0.0.4"): netip.MustParseAddrPort("10.0.0.2:20000"),
				netip.MustParseAddr("10.0.0.5"): netip.MustParseAddrPort("10.0.0.6:60000"),
			},
		},
		{
			//     ✅             ✅            ❌
			// (10.0.0.1) <-> (10.0.0.2) <-> (10.0.0.3)
			name: "Incomplete LSDB",
			lsdb: map[netip.Addr]LSAEntry{
				netip.MustParseAddr(LOCAL_ADDR): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr("10.0.0.2"),
					},
				},
				netip.MustParseAddr("10.0.0.2"): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr("10.0.0.1"),
						netip.MustParseAddr("10.0.0.3"), // 10.0.0.3 is a neighbor of 10.10.10.2 but we don't have 10.0.0.3's LSA yet
					},
				},
			},
			neighborTable: map[netip.Addr]NeighborEntry{
				netip.MustParseAddr("10.0.0.2"): {
					NextHop: netip.MustParseAddrPort("10.0.0.2:20000"),
				},
			},
			expected: map[netip.Addr]netip.AddrPort{
				netip.MustParseAddr("10.0.0.2"): netip.MustParseAddrPort("10.0.0.2:20000"),
			},
		},
		{
			//      Received LSA from 10.10.10.3 --v
			//     ✅             ✅             ✅              ❌             ✅            ✅
			// (10.0.0.1) <-> (10.0.0.2) <-> (10.0.0.3) <-!-> (10.0.0.4) <-> (10.0.0.5) <-> (10.0.0.6)
			name: "Unreachable network",
			lsdb: map[netip.Addr]LSAEntry{
				netip.MustParseAddr(LOCAL_ADDR): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr("10.0.0.2"),
					},
				},
				netip.MustParseAddr("10.0.0.2"): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr("10.0.0.1"),
						netip.MustParseAddr("10.0.0.3"),
					},
				},
				netip.MustParseAddr("10.0.0.3"): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr("10.0.0.2"),
					},
				},
				netip.MustParseAddr("10.0.0.4"): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr("10.0.0.3"),
						netip.MustParseAddr("10.0.0.5"),
					},
				},
				netip.MustParseAddr("10.0.0.5"): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr("10.0.0.4"),
						netip.MustParseAddr("10.0.0.6"),
					},
				},
				netip.MustParseAddr("10.0.0.6"): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr("10.0.0.5"),
					},
				},
			},
			neighborTable: map[netip.Addr]NeighborEntry{
				netip.MustParseAddr("10.0.0.2"): {
					NextHop: netip.MustParseAddrPort("10.0.0.2:20000"),
				},
			},
			expected: map[netip.Addr]netip.AddrPort{
				netip.MustParseAddr("10.0.0.2"): netip.MustParseAddrPort("10.0.0.2:20000"),
				netip.MustParseAddr("10.0.0.3"): netip.MustParseAddrPort("10.0.0.2:20000"),
			},
			expectedUnreachable: []netip.Addr{
				netip.MustParseAddr("10.0.0.4"),
				netip.MustParseAddr("10.0.0.5"),
				netip.MustParseAddr("10.0.0.6"),
			},
		},
		{
			// This case happens when we connect to someone and they send their LSA before we could built our local LSA.
			// This is essentially an error case (we should receive the ACK for our CONN before any LSA).
			// Probably even better to assert that and handle it gracefully before we build the routing table.
			//                                   ✅              ❌
			name: "Only LSA of neighbor", // (10.0.0.2) <->  (10.0.0.1)
			lsdb: map[netip.Addr]LSAEntry{
				netip.MustParseAddr("10.0.0.2"): {
					Neighbors: []netip.Addr{
						netip.MustParseAddr(LOCAL_ADDR),
					},
				},
			},
			neighborTable: map[netip.Addr]NeighborEntry{},
			expected:      map[netip.Addr]netip.AddrPort{},
			expectedUnreachable: []netip.Addr{
				netip.MustParseAddr("10.0.0.2"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			socket := &mockSocket{}
			router := NewRouter(socket)
			router.lsdb = tt.lsdb
			router.neighborTable = tt.neighborTable

			unreachable := router.buildRoutingTable()

			if len(tt.expectedUnreachable) != len(unreachable) {
				t.Errorf("expected %d unreachable addresses, got %d", len(tt.expectedUnreachable), len(unreachable))
			}

			for _, addr := range unreachable {
				if !slices.Contains(tt.expectedUnreachable, addr) {
					t.Errorf("unreachable address %v not in expected unreachable addresses %v", addr, tt.expectedUnreachable)
				}
			}

			if !mapsEqual(router.routingTable, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, router.routingTable)
			}
		})
	}
}

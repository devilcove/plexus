package agent

import (
	"log"
	"net"
	"os"
	"testing"

	"github.com/Kairum-Labs/should"
	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
)

func TestMain(m *testing.M) {
	if _, err := os.Stat("./test.db"); err == nil {
		if err := os.Remove("./test.db"); err != nil {
			log.Println("remove db", err)
			os.Exit(1)
		}
	}
	if err := boltdb.Initialize("./test.db",
		[]string{deviceTable, networkTable},
	); err != nil {
		log.Println("init db", err)
		os.Exit(2)
	}
	plexus.SetLogging("debug")
	code := m.Run()
	// 	cancel()
	// 	wg.Wait()
	boltdb.Close()
	os.Exit(code)
}

func createTestSeverNetworks(t *testing.T) []plexus.Network {
	t.Helper()
	self, err := newDevice()
	should.NotBeError(t, err)
	ip1, cidr1, err := net.ParseCIDR("10.100.0.2/24")
	should.NotBeError(t, err)
	ip2, cidr2, err := net.ParseCIDR("10.200.0.2/24")
	should.NotBeError(t, err)
	return []plexus.Network{
		{
			Name: "one",
			Net:  *cidr1,
			Peers: []plexus.NetworkPeer{
				{
					WGPublicKey: self.WGPublicKey,
					Address: net.IPNet{
						IP:   ip1,
						Mask: net.CIDRMask(32, 32),
					},
				},
			},
		},
		{
			Name: "two",
			Net:  *cidr2,
			Peers: []plexus.NetworkPeer{
				{
					WGPublicKey: self.WGPublicKey,
					Address: net.IPNet{
						IP:   ip2,
						Mask: net.CIDRMask(32, 32),
					},
				},
			},
		},
	}
}

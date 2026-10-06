package agent

import (
	"testing"

	"github.com/Kairum-Labs/should"
	"github.com/vishvananda/netlink"
)

func TestInterface(t *testing.T) {
	deleteAllNetworks()
	deleteAllInterfaces()
	nets := createTestSeverNetworks(t)
	self, err := newDevice()
	should.NotBeError(t, err)
	should.NotBeError(t, saveServerNetworks(self, nets))

	t.Run("startAll", func(t *testing.T) {
		self, err := newDevice()
		should.NotBeError(t, err)
		ifaces, err := netlink.LinkList()
		should.NotBeError(t, err)
		number := len(ifaces)
		startAllInterfaces(self)
		ifaces, err = netlink.LinkList()
		should.NotBeError(t, err)
		should.BeEqual(t, len(ifaces), number+2)
	})
}

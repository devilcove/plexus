package agent

import (
	"testing"

	"github.com/Kairum-Labs/should"
)

func TestSaveServerNetwork(t *testing.T) {
	serverNet := createTestSeverNetworks(t)
	net, err := saveServerNetwork(serverNet[0])
	should.BeNil(t, err)
	should.BeEqual(t, net.Name, serverNet[0].Name)
	dbNet, err := store.Get[Network](net.Name, networkTable)
	should.NotBeError(t, err)
	should.BeEqual(t, dbNet.Name, serverNet[0].Name)
}

func TestSaveServerNetworks(t *testing.T) {
	deleteAllNetworks()
	self, err := newDevice()
	should.NotBeError(t, err)
	serverNets := createTestSeverNetworks(t)
	should.NotBeError(t, saveServerNetworks(self, serverNets))
	networks, err := store.GetAll[Network](networkTable)
	should.NotBeError(t, err)
	should.BeEqual(t, len(networks), 2)
}

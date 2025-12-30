package server

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Kairum-Labs/should"
	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestSubnetRouter(t *testing.T) {
	deleteAllPeers(t)
	deleteAllNetworks(t)
	deleteAllUsers(t)
	createTestNetwork(t)
	peer := createTestNetworkPeer(t)
	user := plexus.User{Username: "test", Password: "pass", IsAdmin: false}
	createTestUser(t, user)

	t.Run("display", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/networks/router/valid/"+peer, nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
	})

	t.Run("badCIDR", func(t *testing.T) {
		payload := bodyParams("cidr", "192.168.0.256/24")
		r := httptest.NewRequest(http.MethodPost, "/networks/router/valid/"+peer, payload)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusBadRequest)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "invalid CIDR address")
	})

	t.Run("badvirtCIDR", func(t *testing.T) {
		payload := bodyParams("cidr", "192.168.0.0/24", "nat", "virt", "vcidr", "192.168.0.256/24")
		r := httptest.NewRequest(http.MethodPost, "/networks/router/valid/"+peer, payload)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusBadRequest)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "invalid CIDR address")
	})

	t.Run("badvirtMask", func(t *testing.T) {
		payload := bodyParams("cidr", "192.168.0.0/24", "nat", "virt", "vcidr", "10.10.10.0/23")
		r := httptest.NewRequest(http.MethodPost, "/networks/router/valid/"+peer, payload)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusBadRequest)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "subnet/virtual subnet masks must be the same")
	})

	t.Run("publicVirtSubnet", func(t *testing.T) {
		payload := bodyParams("cidr", "192.168.0.0/24", "nat", "virt", "vcidr", "8.8.8.8/24")
		r := httptest.NewRequest(http.MethodPost, "/networks/router/valid/"+peer, payload)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusBadRequest)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "invalid subnet: must be a private network")
	})

	t.Run("virtSubnetInUse", func(t *testing.T) {
		_, cidr, err := net.ParseCIDR("10.100.0.0/24")
		should.NotBeError(t, err)
		network := plexus.Network{
			Name: "overlapping",
			Net:  *cidr,
		}
		err = boltdb.Save(network, network.Name, networkTable)
		should.NotBeError(t, err)
		payload := bodyParams("cidr", "192.168.0.0/24", "nat", "virt", "vcidr", "10.100.0.0/24")
		r := httptest.NewRequest(http.MethodPost, "/networks/router/valid/"+peer, payload)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusBadRequest)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "subnet in use")
	})

	t.Run("publicSubnet", func(t *testing.T) {
		payload := bodyParams("cidr", "8.8.8.8/24")
		r := httptest.NewRequest(http.MethodPost, "/networks/router/valid/"+peer, payload)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusBadRequest)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "invalid subnet: must be a private network")
	})

	t.Run("subnetInUse", func(t *testing.T) {
		t.Skip()
	})

	t.Run("goodVirt", func(t *testing.T) {
		setup(t)
		defer shutdown(t)
		payload := bodyParams("cidr", "192.168.0.0/24", "nat", "virt", "vcidr", "10.10.10.0/24")
		r := httptest.NewRequest(http.MethodPost, "/networks/router/valid/"+peer, payload)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "Network:")
	})

	t.Run("goodNat", func(t *testing.T) {
		setup(t)
		defer shutdown(t)
		payload := bodyParams("cidr", "192.168.0.0/24", "nat", "nat", "vcidr", "10.10.10.0/24")
		r := httptest.NewRequest(http.MethodPost, "/networks/router/valid/"+peer, payload)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "Network:")
	})

	t.Run("delete", func(t *testing.T) {
		setup(t)
		defer shutdown(t)
		r := httptest.NewRequest(http.MethodDelete, "/networks/router/valid/"+peer, nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "Network:")
	})
}

func TestSubnetInUse(t *testing.T) {
	public, err := generateKeys()
	should.NotBeError(t, err)
	err = boltdb.Delete[plexus.Network]("plexus", networkTable)
	should.BeTrue(t, err == nil || errors.Is(err, boltdb.ErrNoResults))
	peer := plexus.NetworkPeer{
		WGPublicKey: public.String(),
		HostName:    "peer1",
	}
	network := plexus.Network{
		Name: "plexus",
		Net: net.IPNet{
			IP:   net.ParseIP("10.10.10.0").To4(),
			Mask: net.CIDRMask(20, 32),
		},
	}
	network.Peers = append(network.Peers, peer)
	err = boltdb.Save(network, network.Name, networkTable)
	should.NotBeError(t, err)
	t.Run("overlap network", func(t *testing.T) {
		subnet := &net.IPNet{
			IP:   net.ParseIP("10.10.11.0"),
			Mask: net.CIDRMask(24, 32),
		}
		kind, name, err := subnetInUse(subnet)
		should.BeEqual(t, err, ErrSubnetInUse)
		should.BeEqual(t, kind, "network")
		should.BeEqual(t, name, "plexus")
	})
	t.Run("no subnets", func(t *testing.T) {
		subnet := &net.IPNet{
			IP:   net.ParseIP("192.168.100.0").To4(),
			Mask: net.CIDRMask(24, 32),
		}
		kind, name, err := subnetInUse(subnet)
		should.NotBeError(t, err)
		should.BeEmpty(t, kind)
		should.BeEmpty(t, name)
	})
	t.Run("no overlap", func(t *testing.T) {
		peer.Subnet = net.IPNet{
			IP:   net.ParseIP("192.168.0.0").To4(),
			Mask: net.CIDRMask(20, 32),
		}
		peer.IsSubnetRouter = true
		network.Peers = []plexus.NetworkPeer{peer}
		err = boltdb.Save(network, network.Name, networkTable)
		should.NotBeError(t, err)
		subnet := &net.IPNet{
			IP:   net.ParseIP("10.10.100.0"),
			Mask: net.CIDRMask(24, 32),
		}
		kind, name, err := subnetInUse(subnet)
		should.NotBeError(t, err)
		should.BeEmpty(t, kind)
		should.BeEmpty(t, name)
	})
	t.Run("overlap subnet", func(t *testing.T) {
		subnet := &net.IPNet{
			IP:   net.ParseIP("192.168.1.0").To4(),
			Mask: net.CIDRMask(24, 32),
		}
		kind, name, err := subnetInUse(subnet)
		should.BeEqual(t, err, ErrSubnetInUse)
		should.BeEqual(t, kind, "peer")
		should.BeEqual(t, name, "peer1")
	})
	t.Run("overlap virtual subnet", func(t *testing.T) {
		peer.IsSubnetRouter = true
		peer.UseVirtSubnet = true
		peer.VirtSubnet = net.IPNet{
			IP:   net.ParseIP("172.16.0.0").To4(),
			Mask: net.CIDRMask(20, 32),
		}
		network.Peers = []plexus.NetworkPeer{peer}
		err = boltdb.Save(network, network.Name, networkTable)
		should.NotBeError(t, err)
		subnet := &net.IPNet{
			IP:   net.ParseIP("172.16.1.0"),
			Mask: net.CIDRMask(24, 32),
		}
		kind, name, err := subnetInUse(subnet)
		should.BeEqual(t, err, ErrSubnetInUse)
		should.BeEqual(t, kind, "peer")
		should.BeEqual(t, name, "peer1")
	})
}

// generateKeys generates wgkeys that do not have a / in pubkey.
func generateKeys() (*wgtypes.Key, error) {
	for {
		priv, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return nil, err
		}
		pub := priv.PublicKey()
		if !strings.Contains(pub.String(), "/") {
			return &pub, nil
		}
	}
}

package server

import (
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kairum-Labs/should"
	"github.com/devilcove/boltdb"
	"github.com/devilcove/configuration"
	"github.com/devilcove/mux"
	"github.com/devilcove/plexus"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

var (
	dir      string
	progName = "plexus-testing"
	router   *mux.Router
)

func TestMain(m *testing.M) {
	if _, err := os.Stat("./test.db"); err == nil {
		if err := os.Remove("./test.db"); err != nil {
			log.Println("remove db", err)
			os.Exit(1)
		}
	}
	if err := boltdb.Initialize("./test.db",
		[]string{userTable, keyTable, networkTable, peerTable, settingTable, "keypairs"},
	); err != nil {
		log.Println("init db", err)
		os.Exit(2)
	}
	plexus.SetLogging("debug")
	router = setupRouter(slog.Default())
	code := m.Run()
	// 	cancel()
	// 	wg.Wait()
	boltdb.Close()
	os.Exit(code)
}

func writeTmpConfg(t *testing.T, content *Configuration) {
	t.Helper()
	if err := configuration.Save(content); err != nil {
		t.Fatal(err)
	}
}

func setup(t *testing.T) {
	t.Helper()
	dir = t.TempDir()
	err := os.MkdirAll(filepath.Join(dir, progName), 0o0750)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	os.Args = []string{progName}
	if err := os.MkdirAll(filepath.Join(dir, ".local/share", progName), 0o0750); err != nil {
		t.Fatal(err)
	}
	writeTmpConfg(t, &Configuration{
		FQDN:     "127.0.0.1",
		Port:     "8080",
		DataHome: filepath.Join(dir, ".local/share", progName),
	})
	startTestBroker(t)
}

func deleteAllUsers(t *testing.T) {
	t.Helper()
	users, err := boltdb.GetAll[plexus.User](userTable)
	should.NotBeError(t, err)
	for _, user := range users {
		err := boltdb.Delete[plexus.User](user.Username, userTable)
		should.NotBeError(t, err)
	}
}

func testLogin(t *testing.T, data plexus.User) *http.Cookie {
	t.Helper()
	w := httptest.NewRecorder()
	form := url.Values{}
	form.Add("username", data.Username)
	form.Add("password", data.Password)
	req := httptest.NewRequest(http.MethodPost, "/login/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(w, req)
	should.BeGreaterOrEqualTo(t, len(w.Result().Cookies()), 1)
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "plexus" {
			return cookie
		}
	}
	t.Fail()
	return nil
}

func createTestUser(t *testing.T, user plexus.User) {
	t.Helper()
	user.Password, _ = hashPassword(user.Password)
	err := boltdb.Save(&user, user.Username, userTable)
	should.NotBeError(t, err)
}

func deleteAllPeers(t *testing.T) {
	t.Helper()
	peers, err := boltdb.GetAll[plexus.Peer](peerTable)
	should.NotBeError(t, err)
	for _, peer := range peers {
		err := boltdb.Delete[plexus.Peer](peer.WGPublicKey, peerTable)
		should.NotBeError(t, err)
	}
}

func createTestPeer(t *testing.T) string {
	t.Helper()
	pub, err := generateKeys()
	if err != nil {
		t.Fatal(err)
	}
	savePeer(plexus.Peer{
		WGPublicKey: pub.String(),
		Name:        "testing",
	})
	return pub.String()
}

func createTestNetworkPeer(t *testing.T) string {
	t.Helper()
	id := createTestPeer(t)
	_, err := addPeerToNetwork(id, "valid", 51821, 51821)
	should.NotBeError(t, err)
	return id
}

func deleteAllNetworks(t *testing.T) {
	t.Helper()
	nets, err := boltdb.GetAll[plexus.Network](networkTable)
	should.NotBeError(t, err)
	for _, net := range nets {
		err := boltdb.Delete[plexus.Network](net.Name, networkTable)
		should.NotBeError(t, err)
	}
}

func createTestNetwork(t *testing.T) {
	t.Helper()
	_, cidr, err := net.ParseCIDR("10.200.0.0/24")
	should.NotBeError(t, err)
	network := plexus.Network{
		Name: "valid",
		Net:  *cidr,
	}
	err = boltdb.Save(network, network.Name, networkTable)
	should.NotBeError(t, err)
}

func deleteAllKeys(t *testing.T) {
	t.Helper()
	keys, err := boltdb.GetAll[plexus.Key](keyTable)
	should.NotBeError(t, err)
	for _, key := range keys {
		err := boltdb.Delete[plexus.Key](key.Name, keyTable)
		should.NotBeError(t, err)
	}
}

func startTestBroker(t *testing.T) {
	t.Helper()
	// create admin user.
	adminKey := getAdminKey()
	adminPublicKey, err := adminKey.PublicKey()
	if err != nil {
		t.Fatal("could not create admin public key", "error", err)
	}
	tokensUsers := getTokenUsers()
	deviceUsers := getDeviceUsers()
	natsOptions = &server.Options{
		Nkeys: []*server.NkeyUser{
			{
				Nkey: adminPublicKey,
				Permissions: &server.Permissions{
					Publish: &server.SubjectPermission{
						Allow: []string{">"},
					},
					Subscribe: &server.SubjectPermission{
						Allow: []string{">"},
					},
				},
			},
		},
	}
	natsOptions.Nkeys = append(natsOptions.Nkeys, tokensUsers...)
	natsOptions.Nkeys = append(natsOptions.Nkeys, deviceUsers...)
	natsOptions.NoSigs = true
	natsOptions.Host = "127.0.0.1"
	natServer, err = server.NewServer(natsOptions)
	if err != nil {
		t.Fatal(err)
	}
	natServer.Start()
	SignatureCB := func(nonce []byte) ([]byte, error) {
		return adminKey.Sign(nonce)
	}
	opts := []nats.Option{nats.Nkey(adminPublicKey, SignatureCB)}
	natsConn, err = nats.Connect("nats://127.0.0.1:4222", opts...)
	if err != nil {
		t.Fatal(err)
	}
}

func shutdown(_ *testing.T) {
	natServer.Shutdown()
}

func bodyParams(params ...string) io.Reader {
	body := url.Values{}
	for i := 0; i < len(params)-1; i += 2 {
		body.Set(params[i], params[i+1])
	}
	return strings.NewReader(body.Encode())
}

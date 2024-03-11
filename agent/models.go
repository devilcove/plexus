package agent

import "github.com/devilcove/plexus"

type Network struct {
	plexus.Network
	ListenPort       int
	PublicListenPort int
	Interface        string
	InterfaceSuffix  int
}

type Device struct {
	plexus.Peer
	WGPrivateKey string
	Seed         string
	Server       string
}

type StatusResponse struct {
	Server    string
	Connected string
	Networks  []Network
}

type ServerConnection struct {
	Server    string
	Connected string
}

type NetData struct {
	Name string
}

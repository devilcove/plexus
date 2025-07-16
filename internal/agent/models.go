package agent

import "github.com/devilcove/plexus"

const Agent = "agent"

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
	Connected bool
	Networks  []Network
}

type LeaveServerRequest struct {
	Force bool
}

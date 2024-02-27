package agent

import (
	"log/slog"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
)

func deleteAllNetworks() {
	networks, err := boltdb.GetAll[plexus.Network]("networks")
	if err != nil {
		slog.Error("get networks", "error", err)
	}
	for _, network := range networks {
		if err := boltdb.Delete[plexus.Network](network.Name, "networks"); err != nil {
			slog.Error("delete network", "name", network.Name, "error", err)
		}
	}
}

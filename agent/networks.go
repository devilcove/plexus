package agent

import (
	"log/slog"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
)

func deleteAllNetworks() {
	networks, err := boltdb.GetAll[plexus.Network](networkTable)
	if err != nil {
		slog.Error("get networks", "error", err)
	}
	for _, network := range networks {
		if err := boltdb.Delete[plexus.Network](network.Name, networkTable); err != nil {
			slog.Error("delete network", "name", network.Name, "error", err)
		}
	}
}

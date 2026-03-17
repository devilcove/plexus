package agent

import "runtime/debug"

func Version() string {
	info, _ := debug.ReadBuildInfo()
	return info.Main.Version
}

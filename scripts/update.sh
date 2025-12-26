#!/bin/bash
#
cd ~/sandbox/plexus/app/plexus-server || exit
CGO_ENABLED=0 go build -ldflags "-s -w" -v -o /tmp/plexus-server .
cd ~/sandbox/plexus/app/plexus-agent || exit
CCG_ENABLED=0 go build -ldflags "-s -w" -v -o /tmp/plexus-agent .
cd /tmp || exit
scp plexus-server file.nusak.ca:/srv/http/plexus/plexus-server
scp plexus-agent file.nusak.ca:/srv/http/plexus/plexus-agent


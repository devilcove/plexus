#!/bin/bash
#
cd ~/plexus/app/plexus-server || exit
CGO_ENABLED=0 go build -ldflags "-s -w" -o /tmp/plexus-server-linux-amd64 .
cd ~/plexus/app/plexus-agent || exit
CCG_ENABLED=0 go build -ldflags "-s -w" -o /tmp/plexus-agent-linux-amd64 .
#cd /tmp || exit
#scp plexus-server root@file.nusak.ca:/srv/http
#scp plexus-agent root@file.nusak.ca:/srv/http


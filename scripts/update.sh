#!/bin/bash
#
cd ~/plexus/app/plexus-server
go build -ldflags "-s -w" -o /tmp/plexus-server .
cd ~/plexus/app/plexus-agent
go build -ldflags "-s -w" -o /tmp/plexus-agent .
cd /tmp
scp plexus-server root@file.nusak.ca:/srv/http
scp plexus-agent root@file.nusak.ca:/srv/http


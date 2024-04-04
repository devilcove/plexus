#!/bin/bash
#
cd ~/plexus/app/plexus-server || exit
go build -ldflags "-s -w" -o /tmp/plexus-server .
cd ~/plexus/app/plexus-agent || exit
go build -ldflags "-s -w" -o /tmp/plexus-agent .
cd /tmp || exit
scp plexus-server root@file.nusak.ca:/srv/http
scp plexus-agent root@file.nusak.ca:/srv/http


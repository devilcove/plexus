#!/bin/sh

systemctl stop plexus-server
wget file.nusak.ca/plexus-server -O /tmp/plexus-server
cp /tmp/plexus-server /usr/local/bin/
setcap cap=cap_net_bind_service=ep /usr/local/bin/plexus-server
systemtcl start plexus-server
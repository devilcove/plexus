#!/bin/sh

systemctl stop plexus-server
wget https://github.com/devilcove/plexus/releases/latest/download/plexus-server-linux-amd64 -O /usr/local/bin/plexus-server
setcap cap_net_bind_service=ep /usr/local/bin/plexus-server
systemctl start plexus-server
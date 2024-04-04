#!/bin/sh

systemctl stop plexus-agent
wget https://github.com/devilcove/plexus/releases/latest/download/plexus-agent-linux-amd64 -O /usr/local/bin/plexus-agent
setcap cap_net_admin=ep /usr/local/bin/plexus-agent
systemctl start plexus-agent
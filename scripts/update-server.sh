#!/bin/sh

systemctl stop plexus-server
wget https://file.nusak.ca/plexus/plexus-server -O /usr/local/bin/plexus-server
setcap cap_net_bind_service=ep /usr/local/bin/plexus-server
systemctl start plexus-server
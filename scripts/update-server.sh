#!/bin/sh

systemctl stop plexus-server
wget file.nusak.ca/plexus-server -O /usr/local/bin/plexus-server
setcap cap_net_bind_service=ep /usr/local/bin/plexus-server
systemctl start plexus-server
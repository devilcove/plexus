#!/bin/sh

systemctl stop plexus-agent
wget -4 https://file.nusak.ca/plexus/plexus-agent -O /usr/local/bin/plexus-agent
setcap cap_net_admin=ep /usr/local/bin/plexus-agent
systemctl start plexus-agent
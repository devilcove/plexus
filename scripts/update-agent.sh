#!/bin/sh

systemctl stop plexus-agent
wget file.nusak.ca/plexus-agent -O /usr/local/bin/plexus-agent
setcap cap_net_admin=ep /usr/local/bin/plexus-agent
systemctl start plexus-agent
#!/bin/bash

## check if root
if [ "$(id -u)"  != 0 ]; then
	echo "please run as root"
	exit
fi
echo "Removing plexus agent"
echo ""
systemctl stop plexus-agent
rm /lib/systemd/system/plexus-agent.service
systemctl daemon-reload

echo "deleting files"
rm /usr/local/bin/plexus-agent
rm -r /var/lib/plexus

echo "deleting plexus user"
userdel plexus

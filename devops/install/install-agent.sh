#!/bin/bash


# check if root
if [ "$(id -u)"  != 0 ]; then
	echo "please run as root"
	exit
fi

echo "Installing plexus agent"
echo ""
# create plexus user
echo "creating user plexus"
useradd -r -d /var/lib/plexus -s /sbin/nologin -m plexus

##get files
echo "installing files"
wget https://file.nusak.ca/plexus-agent.service -O /lib/systemd/system/plexus-agent.service
wget https://file.nusak.ca/plexus-agent -O /usr/local/bin/plexus-agent
setcap cap_net_admin=ep /usr/local/bin/plexus-agent
chmod +x /usr/local/bin/plexus-agent

echo "installing systemd service"
systemctl daemon-reload
systemctl enable plexus-agent
systemctl start plexus-agent
sleep 2
systemctl status plexus-agent --no-pager -l

echo "plexus-agent is installed and running"
plexus-agent -h



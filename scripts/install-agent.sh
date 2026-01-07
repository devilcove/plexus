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
wget -4 https://raw.githubusercontent.com/devilcove/plexus/master/files/plexus-agent.service -O /lib/systemd/system/plexus-agent.service
wget -4 https://file.nusak.ca/plexus/plexus-agent -O /usr/local/bin/plexus-agent
setcap cap_net_admin=ep /usr/local/bin/plexus-agent
chmod +x /usr/local/bin/plexus-agent
install -o plexus -g plexus -d /var/lib/plexus/.local/share/plexus-agent
cd /var/lib/plexus
chown -R plexus:plexus .local/share/

echo "installing systemd service"
systemctl daemon-reload
systemctl enable plexus-agent
systemctl start plexus-agent
sleep 2
systemctl status plexus-agent --no-pager -l

echo "plexus-agent is installed and running"
plexus-agent -h



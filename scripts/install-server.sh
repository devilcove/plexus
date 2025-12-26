#!/bin/bash

get_passwd() {
    echo "Enter password for plexus administrator"
    echo "    Press enter to use defaults"
    read pass1
    echo "Retype plexus administrator password"
    echo "    Press enter to use defaults"
    read pass2
}

set -e 
#check if root
if [ "$(id -u)" != 0 ]; then 
    echo "please run as root"
    exit
fi
echo "installing plexus server"
echo ""
# create plexus user
echo "creating user plexus"
useradd -r -d /var/lib/plexus -G systemd-journal -s /sbin/nologin -m plexus

##get files
echo "installing files"
wget -4 https://raw.githubusercontent.com/devilcove/plexus/master/files/plexus-agent.service -O /lib/systemd/system/plexus-agent.service
wget -4 https://file.nusak.ca/plexus/plexus-agent -O /usr/local/bin/plexus-agent
wget -4 https://raw.githubusercontent.com/devilcove/plexus/master/files/plexus-server.service -O /lib/systemd/system/plexus-server.service
wget -4 https://file.nusak.ca/plexus/plexus-server -O /usr/local/bin/plexus-server
setcap cap_net_admin=ep /usr/local/bin/plexus-agent
setcap cap_net_bind_service=ep /usr/local/bin/plexus-server
chmod +x /usr/local/bin/plexus-agent
chmod +x /usr/local/bin/plexus-server
install -o plexus -g plexus -d /var/lib/plexus/.config/plexus-server
install -o plexus -g plexus -d /var/lib/plexus/.local/share/plexus-server
install -o plexus -g plexus -d /var/lib/plexus/.local/share/plexus-agent
cd /var/lib/plexus
chown plexus:plexus .config
chown -R plexus:plexus .local/share/

#get input
echo "Enter Fully Qualified Domain Name of plexus server (eg. plexus.example.com)"
read fqdn
echo "Enter email to use with letsencrypt"
read email
echo "Enter name for plexus administrator"
    echo "    Press enter to use defaults"
read user
get_passwd
while [ "$pass1" != "$pass2" ]
do
    echo "passwords do not match ... try again"
    echo ""
    get_passwd
done

cat << EOF > /tmp/plexus.config
fqdn: $fqdn
email: $email
adminname: $user
adminpass: $pass1
secure: true
EOF

install -o plexus -g plexus -D /tmp/plexus.config /var/lib/plexus/.config/plexus-server/config

set +e
echo "installing systemd service"
systemctl daemon-reload
systemctl enable plexus-server
systemctl enable plexus-agent
systemctl start plexus-server
systemctl start plexus-agent
systemctl status plexus-server --no-pager -l
systemctl status plexus-agent --no-pager -l

echo ""
echo "plexus-agent and plexus-server are installed"
echo "log into the server at https://$fqdn"

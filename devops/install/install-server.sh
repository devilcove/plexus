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
useradd -r -d /var/lib/plexus -s /sbin/nologin -m plexus

##get files
echo "installing files"
wget https://file.nusak.ca/plexus-agent.service -O /lib/systemd/system/plexus-agent.service
wget https://file.nusak.ca/plexus-server.service -O /lib/systemd/system/plexus-server.service
wget https://file.nusak.ca/plexus-agent -O /usr/local/bin/plexus-agent
wget https://file.nusak.ca/plexus-server -O /usr/local/bin/plexus-server
setcap cap_net_admin=ep /usr/local/bin/plexus-agent
setcap cap_net_bind_service=ep /usr/local/bin/plexus-server
chmod +x /usr/local/bin/plexus-agent
chmod +x /usr/local/bin/plexus-server
mkdir /etc/plexus
chown plexus:plexus /etc/plexus

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

cat << EOF > /etc/plexus/config
fqdn: $fqdn
email: $email
adminname: $user
adminpass: $pass1
EOF

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
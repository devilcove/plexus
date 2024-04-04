Plexus
======
![tests](https://github.com/devilcove/plexus/actions/workflows/integrationtest.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/devilcove/pleuxs?style=flat-square)](https://goreportcard.com/report/github.com/devilcove/plexus)
[![Go Reference](https://pkg.go.dev/badge/github.com/devilcove/plexus.svg)](https://pkg.go.dev/github.com/devilcove/plexus)
[![Go Coverage](https://raw.githubusercontent.com/wiki/devilcove/plexus/coverage/coverage.svg)](https://raw.githack.com/wiki/devilcove/plexus/coverage.html)

Plexus provides tools to easily manage private wireguard networks.

Overview
========
This repository provides the code for plexus-server and plexus-agent.
Plexus-server serves the web ui and controls communications with plexus agents for peer and network updates.
Plexus-agent runs as a daemon to communicate with the plexus server for peer and network updates.  It also provided a cli interface to register and leave servers and to join/leave networks.

QuickStart
==========
Server
------
* Requirements
    * linux VPS with minimum 1CPU/1GB memory (eg.  $6 [DigitalOcean droplet](https://m.do.co/c/17e44f225e2e))
    * DNS record pointing to the IP of your server
    * systemd
    * tcp ports 443 and 4222  -- for server
    * udp ports 51820 -- for agent: additional ports are needed for each network 
* Download [install script](https://raw.githubusercontent.com/devilcove/plexus/master/scripts/install-server.sh)
* make script executable (chmod +x install-server.sh)
* run script  (as root)
    * script will ask for
        * FQDN of server
        * email to use with [Let's Encrypt](https://letsencrypt.org)
        * name/password for web ui admin user
    * script will install and start plexus-server and plexus-agent daemons
* log into server web ui
* create registration key via web ui
* create network via web ui
* register agent(s) with server vi agent cli
* join peer to network :  can use server web ui or agent cli

Agent
-----
* Requirements
    * Linux host with systemd
* Download [install script](https://raw.githubusercontent.com/devilcove/plexus/master/scripts/install-agent.sh)
* make script executable (chmod +x install-server.sh)
* run script  (as root)
    * script will install and start plexus-agent daemon
* copy registration key from server
* register with server and join network
```
    plexus-agent register <registraton key>
    plexus-agent join <network name>
```

Tech Stack
==========
* Language: [Go](https://go.dev)
* HTTP Framework: [Gin](https://github.com/gin-gonic/gin)
* FrontEnd Framework: html/[htmx](https://htmx.org)
* CSS Library: [w3schools](https://w3schools.com/w3css)
* Database(key/value): [bbolt](https://github.com/etcd-io/bbolt)
* Pub/Sub Broker: [nats.io](https://nats.io)
* Automatic SSL Certificates: [certmagic](https://github.com/caddyserver/certmagic)

Docs
====
### [Installation](docs/install.md)
### [Configuration](docs/configuration.md)
### [Security](docs/security.md)
### [Server](docs/server.md)
### [Agent](docs/agent.md)

Legal
=====
WireGuard and the WireGuard logo are registered trademarks of Jason A. Donenfeld.

Support
=======
Please use [Issues](https://github.com/devilcove/plexus/issues) or [Discussions](https://github.com/devilcove/plexus/discussions) for support requests

![<img src="https://img.buymeacoffee.com/button-api/?text=Buy me a coffee&emoji=&slug=mkasun&button_colour=5F7FFF&font_colour=ffffff&font_family=Comic&outline_colour=000000&coffee_colour=FFDD00"](https://www.buymeacoffee.com/mkasun)
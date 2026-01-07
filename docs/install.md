INSTALLATION
------------
Scripts
=======
Installation, Uninstallation and Update scripts for both server and agent are provided in the [scripts dir](https://github.com/devilcove/plexus/tree/master/scripts)

The scripts need to be run as root

### Install
The server install script installs plexus-server and plexus-agent.

The agent install script installs plexus-agent only

The install scripts:
* collects configuration information (server only)
    * Fully Qualified Domain Name of server
    * Email address for [Let's Encrytpt](https://letsencrypt.com)
    * Username/Password for default user

> collected information is not validated by install script but will be validated on server startup

* downloads binary executable(s) and systemd service files
* sets required capabilities on binary executable(s)
* creates a new system user (plexus)
* enables and starts the systemd service(s)

### Update
The update scripts:
* stops systemd service
* downloads updated versions of binary
* sets required capabilities on binary
* starts systemd service

### Uninstall
The server uninstall script uninstalls plexus-server and plexus-agent.

The agent uninstall script uninstalls plexus-agent only

The uninstall scripts:
* stops systemd service(s)
* removes systemd service(s)
* deletes binary and config/data directories
* deletes plexus user

Disable TLS
===========
If the server is to be installed behind a reverse proxy or in a lan only setting, tls can be disabled.  Both web and nats traffic will be not be encrypted.
### Configuration
* [see configuration settings](configuration.md)
* set secure = false
* set port (default 8080) to desired listening port for web traffic
* fqdn must be set (can be an IP address) such that all peers can resolve the address of the server
> setting fqdn to an IP address is not valid with secure=true.  Let's Encrypt does not support certificates for [IP addresses](https://community.letsencrypt.org/t/ssl-on-a-ip-instead-of-domain/90635)

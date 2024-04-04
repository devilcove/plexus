Security
========
TLS
---
Transport Level Security is used for web and nats server endpoints. [Certmagic](https://github.com/caddyserver/certmagic) is used to obtain certificates via [Let's Encrypt](https://letsencrypt.com)

TLS security can be turned off by setting the configuration variable secure=false.  For example, if the plexus server is to be installed behind a reverse proxy. [See](install.md) for more information.

Users
-----
Web ui users passwords are hashed with [bcrypt](https://pkg.go.dev/golang.org/x/
crypto@v0.21.0/bcrypt)

### Initial User
On startup, plexus-server checks to see that at least one user with admin permission exists. If not, one is created with username/password from config file or default values.   

### User Types
#### Admin
Admin users have the additional capability to create new users and edit existing users
#### Normal
Normal users can only edit their own user account (eg. change password). Normal users cannot create new users nor edit other users.

NATS
----
In addition to TLS security between agent(s) and server, Nkeys (a new, highly secure public-key signature system based on Ed25519) are used to control access to publication and subscription topics.

Once an agent has joined a server it can
* Publish
    * to topics beginning with its ID (WGPublicKey)
* Subcribe
    * to topics beginning with its ID (WGPublicKey)
    * all network updates    

Nats is also used for communications between the plexus-agent cli and the plexus-agent daemon. The connection is not encrypted with TLS but the daemon only listens for NATS connections on localhost so only nats clients on the same host can connect.

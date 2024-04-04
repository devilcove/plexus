Configuration
=============
Order of precedence
1. Environment variable with prefix of PLEXUS_
2. Configuration variables in file /etc/plexus/config.yml 
3. Default values


Server
------
| Variable  | Env     | Default  |  Usage |
| --- | ---- | ---- | --- | 
| adminname | PLEXUS_ADMINNAME | admin | default admin username |
| adminpass | PLEXUS_ADMINPASS | password | password for default admin |
| verbosity | PLEXUS_VERBOSITY | info |  level for logs |
| secure | PLEXUS_SECURE | true | use TLS for http and nats |
| port | PLEXUS_PORT | 8080 | web listen port when secure is false |
| email | PLEXUS_EMAIL | | email for use with Let's Encrypt |

* adminname/adminpass is only used to create a default user iff an admin user does not exist on server startup

Agent
-----
| Variable  | Env     | Default  |  Usage |
| --- | ---- | ---- | --- | 
| verbosity | PLEXUS_VERBOSITY | info |  level for logs |
| natsport | PLEXUS_NATSPORT | 4223 | port for cli <-> agent nats comms |
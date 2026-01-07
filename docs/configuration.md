Configuration
=============


Server
------
| Variable  | Default  |  Usage |
| --- |  ---- | --- | 
| adminname | admin | default admin username |
| adminpass | password | password for default admin |
| verbosity | info |  level for logs |
| secure |  true | use TLS for http and nats |
| port |  | 8080 | web listen port when secure is false |
| email |  | email for use with Let's Encrypt |

* adminname/adminpass is only used to create a default user iff an admin user does not exist on server startup

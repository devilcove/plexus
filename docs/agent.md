Plexus Agent CLI
----------------

plexus-agent commands can be run by an ordinary user with exception of run commmand (agent daemon) which is intended to be run by plexus user
```
plexus agent to setup and manage plexus wireguard
networks.  Communicates with plexus server for network updates.
CLI to join/leave networks.

Usage:
  plexus-agent [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  drop        unregister from server
  help        Help about any command
  join        join network
  leave       leave network
  loglevel    set log level of daemon (error, warn, info, debug)
  register    register with a plexus server
  reload      reload network configuration(s)
  reset       reset interface peers for specified network
  run         plexus-agent deamon
  status      display status
  version     display version information

Flags:
      --config string      config file (default is /etc/plexus/plexus-agent.yaml)
  -h, --help               help for plexus-agent
  -v, --verbosity string   logging verbosity (default "INFO")

Use "plexus-agent [command] --help" for more information about a command.
```
Register
========
The register command registers a peer with the plexus server.
To use, copy a registration key token from the plexus server
and run command

``` plexus-agent register <token> ```
```
register with a plexus server using token

Usage:
  plexus-agent register token [flags]

Flags:
  -h, --help   help for register

Global Flags:
      --config string      config file (default is /etc/plexus/plexus-agent.yaml)
  -v, --verbosity string   logging verbosity (default "INFO")
```

Drop
====
The drop command will delete all networks and drop registration with server
```
unregister from server. Also deletes networks controlled by server

Usage:
  plexus-agent drop [flags]

Flags:
  -h, --help   help for drop

Global Flags:
      --config string      config file (default is /etc/plexus/plexus-agent.yaml)
  -v, --verbosity string   logging verbosity (default "INFO")
```

Join
====
Join command joins an existing network
```
join network

Usage:
  plexus-agent join network [flags]

Flags:
  -h, --help   help for join

Global Flags:
      --config string      config file (default is /etc/plexus/plexus-agent.yaml)
  -v, --verbosity string   logging verbosity (default "INFO")
```

Leave
=====
Leave network command deletes network on peer
```
Usage:
  plexus-agent leave network [flags]

Flags:
  -h, --help   help for leave

Global Flags:
      --config string      config file (default is /etc/plexus/plexus-agent.yaml)
  -v, --verbosity string   logging verbosity (default "INFO")
Leave command deletes current network on peer
```

Status
======
Status command displays infomation about server and networks/wireguard interfaces. The server information includes server endpoint and connectivity status.  The network/wireguard interface information is displayed in a format similar to ```wg show``` but with additional information about networks/interfaces and peers.

Networks additional information
* network name
* network address
* public listen port

Peers additional information
* peer name
* peer address
```
~> plexus-agent status
Server
	 nats://plexus.nusak.ca:4222 : true

interface: plexus0
	 network name: plexus
	 public key: p1AvfOzFgL2nEJrr8pvBeqEt+DPWGrcBfdHfKivjqVk=
	 listen port: 51820
	 public listen port: 51820
	 address: 10.10.10.1
peer: WhpdSseydj0jpJcNqsnzt8PZ93FUlpKFbR4ZyoUpVDo= winterfell 10.10.10.2
	endpoint: 129.222.192.188: 30403
	allowed ips: 10.10.10.2/32
	last handshake: 117.5493160.0 seconds ago
	transfer: 9980 sent 10328 received
	keepalive: 20s
peer: pG8tT7Yj0e50JAk9MQw3vZuuN256ispkOfXDzzHC+kI= firefly 10.10.10.3
	endpoint: 140.238.132.144: 51820
	allowed ips: 10.10.10.3/32 10.225.211.0/24
	last handshake: 104.2563300.0 seconds ago
	transfer: 3216 sent 12336 received
	keepalive: 20s
peer: vrn9w9h8CwmZ++M7BS6EBDSpUUdu+gOXPmp7lZX37m0= oracle 10.10.10.4
	endpoint: 150.230.31.118: 51820
	allowed ips: 10.10.10.4/32
	last handshake: 37.8089930.0 seconds ago
	transfer: 12364 sent 3272 received
	keepalive: 20s
```  
Reload
======
Reload commands fetches fresh data from plexus server and reinitializes wireguard interfaces.
```
reload network configurations(s)

Usage:
  plexus-agent reload [flags]

Flags:
  -h, --help   help for reload

Global Flags:
      --config string      config file (default is /etc/plexus/plexus-agent.yaml)
  -v, --verbosity string   logging verbosity (default "INFO")
```

Reset
=====
Reset command resets the wireguard interface for a given network
```
resets wg interface peers for given network

Usage:
  plexus-agent reset network [flags]

Flags:
  -h, --help   help for reset

Global Flags:
      --config string      config file (default is /etc/plexus/plexus-agent.yaml)
  -v, --verbosity string   logging verbosity (default "INFO")
```

Version
=======
Version command displays version information:
* semantic version number
* git commit hash
* local modifications to commit

-l --long flags displays version infomation for
* current binary
* agent daemon
* server
```
plexus-agent version
v0.1.0: git 2dee6a3b1e8551dc637bcbecd62d1a298b9c3f1f 2024-03-25T11:38:43Z true

plexus-agent version -l
Server: v0.1.0: git 873255e5f846292c787fe4e332818592d6991baa 2024-03-31T13:51:06Z false
Agent:  v0.1.0: git 2dee6a3b1e8551dc637bcbecd62d1a298b9c3f1f 2024-03-25T11:38:43Z true
Binary: v0.1.0: git 2dee6a3b1e8551dc637bcbecd62d1a298b9c3f1f 2024-03-25T11:38:43Z true
```

LogLevel
========
LogLevel command sets the logging level of agent daemon
```
plexus-agent loglevel -h
set log level of damemon
debug, info, warn, or error
.

Usage:
  plexus-agent loglevel level [flags]

Flags:
  -h, --help   help for loglevel

Global Flags:
      --config string      config file (default is /etc/plexus/plexus-agent.yaml)
  -v, --verbosity string   logging verbosity (default "INFO")
```

Run
===
Run command stars the plexus-agent daemon.  It is intended to be called as systemd service.  If it is run as an ordinary user it will fail with permission errors.  




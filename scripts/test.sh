#1/bin/bash

mkdir /tmp/plexus

go test -c  -o /tmp/plexus ./...

path="/tmp/plexus"

for file in "$path"/*; do
    sudo setcap cap_net_admin=ep $file
    $file -test.v -test.timeout=10s
done

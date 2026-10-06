#1/bin/bash

mkdir /tmp/plexus
mkdir /tmp/coverage

go test -c  -coverpkg=./... -o /tmp/plexus ./...

path="/tmp/plexus"

for file in "$path"/*; do
    sudo setcap cap_net_admin=ep $file
    $file -test.v -test.gocoverdir=/tmp/coverage -test.timeout=10s
done

go tool covdata textfmt -i=/tmp/coverage -o=/tmp/coverage/coverage.out

go tool cover -html=/tmp/coverage/coverage.out

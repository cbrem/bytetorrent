#!/bin/bash

# Exit if GOPATH is not set.
if [ -z $GOPATH ]; then
    echo "FAIL: GOPATH environment variable is not set"
    exit 1
fi

if [ -z $GOBIN]; then
    export GOBIN=$GOPATH/bin
fi

go install runners/trackerrunner/tracker_runner.go
go install tests/trackertest/trackertest.go

$GOBIN/trackertest

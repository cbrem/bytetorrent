#!/bin/bash

# Exit if GOPATH is not set.
if [ -z $GOPATH ]; then
    echo "FAIL: GOPATH environment variable is not set"
    exit 1
fi

# Exit if OS is not supported.
if [ -n "$(go version | grep 'darwin/amd64')" ]; then    
    GOOS="darwin_amd64"
elif [ -n "$(go version | grep 'linux/amd64')" ]; then
    GOOS="linux_amd64"
else
    echo "FAIL: only 64-bit Mac OS X and Linux operating systems are supported"
    exit 1
fi

# Build the command-line client.
# Exit immediately if there was a compile-time error.
go install runners/client
if [ $? -ne 0 ]; then
   echo "FAIL: code does not compile"
   exit $?
fi

# Build the dummy tracker implementation.
# Exit immediately if there was a compile-time error.
go install runners/dummytracker
if [ $? -ne 0 ]; then
   echo "FAIL: code does not compile"
   exit $?
fi

# Pick random ports between [10000, 20000).
CLIENT_PORT=$(((RANDOM % 10000) + 10000))
TRACKER_PORT=$(((RANDOM % 10000) + 10000))

# Binaries
CLIENT=$GOPATH/bin/client
TRACKER=$GOPATH/bin/dummytracker

# Create pipe for input to client.
PIPE=tmp
mkfifo $PIPE

# Start tracker.
${TRACKER} "localhost:${TRACKER_PORT}" &
TRACKER_PID=$!
sleep 2

# Start client.
${CLIENT} "localhost:${CLIENT_PORT}" "localhost:${TRACKER_PORT}" < $PIPE &
CLIENT_PID=$!
sleep 2

# Exit client using its interface.
echo -e "EXIT" > $PIPE
sleep 2

# Kill tracker.
kill -9 ${TRACKER_PID}
sleep 2

# Clean up temporary files.
rm $PIPE

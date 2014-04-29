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

# Binaries
CLIENT=$GOPATH/bin/client
TRACKER=$GOPATH/bin/dummytracker

# Identify the file for torrenting.
# It should already exist on disk. If not, exit.
FILE_PATH=music.mp3
if [ -f ${FILE_PATH} ];
then
  echo "Source file exists."
else
   echo "Source file ${FILE_PATH} does not exist."
   exit $?
fi

# Start tracker.
TRACKER_PORT=$(((RANDOM % 10000) + 10000))
${TRACKER} "localhost:${TRACKER_PORT}" &
TRACKER_PID=$!
sleep 2

# Start clients.
NUM_CLIENTS=10
PIPE_BASE=/tmp/pipe
for ((i=0; i<$NUM_CLIENTS; i++))
do
  PORT=$(((RANDOM % 10000) + 10000))
  PIPE="${PIPE_BASE}${i}"
  mkfifo ${PIPE}
  ${CLIENT} "localhost:${PORT}" "localhost:${TRACKER_PORT}" < ${PIPE} &
done
sleep 2

# Create and register a torrent once.
# Do this using client 1.
TORRENT_NAME=music
CREATOR=1
echo -e "CREATE ${FILE_PATH} ${TORRENT_NAME}" >> "${PIPE_BASE}${CREATOR}"
sleep 2
echo -e "REGISTER ${TORRENT_NAME}.torrent" >> "${PIPE_BASE}${CREATOR}"
sleep 2

# Offer the torrent from all clients 1, ..., $NUM_CLIENT-1
for ((i=1; i<$NUM_CLIENTS; i++))
do
  echo -e "OFFER ${FILE_PATH} ${TORRENT_NAME}.torrent" >> "${PIPE_BASE}${i}"
done
sleep 2

# Download the file from client0.
DOWNLOADED_FILE_PATH=downloaded_music.mp3
DOWNLOADER=0
echo -e "DOWNLOAD ${DOWNLOADED_FILE_PATH} ${TORRENT_NAME}.torrent" >> "${PIPE_BASE}${DOWNLOADER}"
sleep 45

# Exit all clients.
for ((i=0; i<$NUM_CLIENTS; i++))
do
  PIPE="${PIPE_BASE}${i}"
  echo -e "EXIT" >> $PIPE
done
sleep 2

# Kill tracker.
kill -9 ${TRACKER_PID}
sleep 2

# Check that source file and downloaded file are identical.
SOURCE_FILE_CONTENTS=$(<${FILE_PATH})
DOWNLOADED_FILE_CONTENTS=$(<${DOWNLOADED_FILE_PATH})
if [ "${SOURCE_FILE_CONTENTS}" = "${DOWNLOADED_FILE_CONTENTS}" ];
then echo "PASS"
else echo "FAIL"
fi

# Clean up temporary files.
for ((i=0; i<$NUM_CLIENTS; i++))
do
  PIPE="${PIPE_BASE}${i}"
  rm $PIPE
done
rm "${TORRENT_NAME}.torrent"

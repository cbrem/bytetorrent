#!/bin/bash

# Variables.
CLIENT=$GOPATH/bin/client
CLIENT_PRETTY_PRINT=no
TRACKER=$GOPATH/bin/trackerrunner
NUM_GOOD_CLIENTS=5
NUM_BAD_CLIENTS=5
NUM_CLIENTS=$NUM_GOOD_CLIENTS+$NUM_BAD_CLIENTS
CLIENT_PIPE_BASE=/tmp/client_pipe

FILE_PATH=music.mp3
BAD_FILE_PATH=virus.mp3

NUM_TRACKERS=3
TRACKER_PIPE_BASE=/tmp/tracker_pipe

TORRENT_NAME=music
CREATOR=1

DOWNLOADED_FILE_PATH=downloaded_music.mp3
DOWNLOADER=0

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

# Build the tracker implementation.
# Exit immediately if there was a compile-time error.
go install runners/trackerrunner
if [ $? -ne 0 ]; then
   echo "FAIL: code does not compile"
   exit $?
fi

# Identify the file for torrenting.
# It should already exist on disk. If not, exit.
if [ -f ${FILE_PATH} ];
then
  echo "Source file exists."
else
   echo "Source file ${FILE_PATH} does not exist."
   exit $?
fi

# Identify the bad file.
# It should already exist on disk. If not, exit.
if [ -f ${BAD_FILE_PATH} ];
then
  echo "Fake file exists."
else
   echo "Fake file ${BAD_FILE_PATH} does not exist."
   exit $?
fi

# Start trackers.
TRACKER_HOST_PORTS=""
MASTER_PORT=""
for ((i=0; i<$NUM_TRACKERS; i++))
do
  PORT=$(((RANDOM % 10000) + 10000))
  TRACKER_HOST_PORTS="${TRACKER_HOST_PORTS} localhost:${PORT}"
  PIPE="${TRACKER_PIPE_BASE}${i}"
  mkfifo ${PIPE}

  if [ ${i} = 0 ];
  then
    ${TRACKER} ${PORT} ${NUM_TRACKERS} ${i} < ${PIPE}&
    MASTER_PORT=${PORT}
  else
    ${TRACKER} ${PORT} ${NUM_TRACKERS} ${i} localhost:${MASTER_PORT} < ${PIPE} &
  fi

  echo -e "\n" >> ${PIPE}
done
sleep 5

# Start clients.
for ((i=0; i<$NUM_CLIENTS; i++))
do
  PORT=$(((RANDOM % 10000) + 10000))
  PIPE="${CLIENT_PIPE_BASE}${i}"
  mkfifo ${PIPE}
  ${CLIENT} ${CLIENT_PRETTY_PRINT} "localhost:${PORT}" ${TRACKER_HOST_PORTS} < ${PIPE} &
done
sleep 5

# Create and register a torrent once.
# Do this using client 1.
echo -e "CREATE ${FILE_PATH} ${TORRENT_NAME}" >> "${CLIENT_PIPE_BASE}${CREATOR}"
sleep 2
echo -e "REGISTER ${TORRENT_NAME}.torrent" >> "${CLIENT_PIPE_BASE}${CREATOR}"
sleep 2

# Offer the torrent from all clients 1, ..., $NUM_CLIENT-1
for ((i=1; i<$NUM_GOOD_CLIENTS; i++))
do
  echo -e "OFFER ${FILE_PATH} ${TORRENT_NAME}.torrent" >> "${CLIENT_PIPE_BASE}${i}"
done
for ((i=$NUM_GOOD_CLIENTS; i<$NUM_CLIENTS; i++))
do
  echo -e "OFFER ${BAD_FILE_PATH} ${TORRENT_NAME}.torrent" >> "${CLIENT_PIPE_BASE}${i}"
done
sleep 15

# Download the file from client0.
echo -e "DOWNLOAD ${DOWNLOADED_FILE_PATH} ${TORRENT_NAME}.torrent" >> "${CLIENT_PIPE_BASE}${DOWNLOADER}"
sleep 5

# Exit all clients.
for ((i=0; i<$NUM_CLIENTS; i++))
do
  PIPE="${CLIENT_PIPE_BASE}${i}"
  echo -e "EXIT" >> $PIPE
done
sleep 2

# Exit all trackers.
for ((i=0; i<$NUM_TRACKERS; i++))
do
  PIPE="${TRACKER_PIPE_BASE}${i}"
  echo -e "0" >> $PIPE
done
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
  PIPE="${CLIENT_PIPE_BASE}${i}"
  rm $PIPE
done
for ((i=0; i<$NUM_TRACKERS; i++))
do
  PIPE="${TRACKER_PIPE_BASE}${i}"
  rm $PIPE
done
rm "${TORRENT_NAME}.torrent"

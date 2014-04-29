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

# Create a file for torrenting.
FILE_PATH=/tmp/foo.txt
FILE_CONTENTS="test file"
echo -e ${FILE_CONTENTS} > ${FILE_PATH}

# Start tracker.
TRACKER_PORT=$(((RANDOM % 10000) + 10000))
${TRACKER} "localhost:${TRACKER_PORT}" &
TRACKER_PID=$!
sleep 2

# Start client1.
CLIENT1_PORT=$(((RANDOM % 10000) + 10000))
PIPE1=/tmp/tmp1
mkfifo $PIPE1
${CLIENT} "localhost:${CLIENT1_PORT}" "localhost:${TRACKER_PORT}" < $PIPE1 &
sleep 2

# Start client2.
CLIENT2_PORT=$(((RANDOM % 10000) + 10000))
PIPE2=/tmp/tmp2
mkfifo $PIPE2
${CLIENT} "localhost:${CLIENT2_PORT}" "localhost:${TRACKER_PORT}" < $PIPE2 &
sleep 2

# Create, register, and upload a torrent on client1.
TORRENT_NAME=foo
echo -e "CREATE ${FILE_PATH} ${TORRENT_NAME}" >> $PIPE1
sleep 2
echo -e "REGISTER ${TORRENT_NAME}.torrent" >> $PIPE1
sleep 2
echo -e "OFFER ${FILE_PATH} ${TORRENT_NAME}.torrent" >> $PIPE1
sleep 2

# Download the file from client2.
DOWNLOADED_FILE_PATH=/tmp/downloaded_foo.txt
echo -e "DOWNLOAD ${DOWNLOADED_FILE_PATH} ${TORRENT_NAME}.torrent" >> $PIPE2
sleep 2

# Exit client.
echo -e "EXIT" >> $PIPE1
echo -e "EXIT" >> $PIPE2
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
rm ${PIPE1}
rm ${PIPE2}
rm ${FILE_PATH}
rm ${DOWNLOADED_FILE_PATH}
rm "${TORRENT_NAME}.torrent"

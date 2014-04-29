package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"regexp"
	"strconv"
	"time"
	"torrent"
	"torrent/torrentproto"
	"tracker"
	"tracker/trackerproto"
)

type trackerTester struct {
	tracker tracker.Tracker
	srv     *rpc.Client
}

type testFunc struct {
	name string
	f    func()
}

var LOGE = log.New(os.Stderr, "", log.Lshortfile|log.Lmicroseconds)

func createTracker(master string, numNodes, port, nodeId int) (*trackerTester, error) {
	tracker, err := tracker.NewTrackerServer(master, numNodes, port, nodeID)
	if err != nil {
		LOGE.Println("Could not create tracker")
		return nil, err
	}

	srv, err := rpc.DialHTTP("tcp", net.JoinHostPort("localhost", strconv.Itoa(port)))
	if err != nil {
		LOGE.Println("Could not connect to tracker")
		return nil, err
	}

	return &trackerTester{tracker: tracker, srv: srv}, nil
}

func (t *trackerTester) GetOpt(seqNum int) (*trackerproto.GetReply, error) {
	args := &trackerproto.GetArgs{SeqNum: seqNum}
	reply := &trackerproto.GetReply{}
	err := t.srv.Call("PaxosTracker.GetOpt", args, reply)
	return reply, err
}

func (t *trackerTester) ConfirmChunk(chunk torrentproto.ChunkID, hostPort string) (*trackerproto.UpdateReply, error) {
	args := &trackerproto.ConfirmArgs{
		Chunk: chunk,
		HostPort: hostPort}
	reply := &trackerproto.UpdateReply
	err := t.srv.Call("RemoteTracker.ConfirmChunk", args, reply)
	return reply, err
}

func (t *trackerTester) RequestChunk(chunk torrentproto.ChunkID) (*trackerproto.RequestReply, error) {
	args := &trackerproto.RequestArgs{Chunk: chunk}
	reply := &trackerproto.RequestReply
	err := t.srv.Call("RemoteTracker.RequestChunk", args, reply)
	return reply, err
}

func (t *trackerTester) CreateEntry(torrent torrentproto.Torrent) (*trackerproto.UpdateReply, error) {
	args := &trackerproto.CreateArgs{Torrent: torrent}
	reply := &trackerproto.UpdateReply
	err := t.srv.Call("RemoteTracker.CreateEntry", args, reply)
	return reply, err
}

func (t *trackerTester) GetTrackers() (*trackerproto.TrackersReply, error) {
	args := &trackerproto.TrackersArgs{}
	reply := &trackerproto.TrackersReply{}
	err := t.srv.Call("RemoteTracker.GetTrackers", args, reply)
	return reply, err
}

func (t *trackerTester) ReportMissing(chunk torrentproto.ChunkID, hostPort string) (*trackerproto.UpdateReply, error) {
	args := &trackerproto.ReportArgs{
		Chunk: chunk,
		HostPort: hostPort}
	reply := &trackerproto.UpdateReply{}
	err := t.srv.Call("RemoteTracker.ReportMissing", args, reply)
	return reply, err
}

// returns a torrent object with the provided info
// if trackersGood is false, then it just makes up trackers
// if trackersGood is true, then it gets the trackers from t
func newTorrentInfo(t *trackerTester, trackersGood bool, numChunks int) (torrentproto.Torrent, error) {
	if trackersGood {
		trackers, err := t.GetTrackers()
		if err != nil || trackers.Status != trackerproto.OK {
			LOGE.Println("Get Trackers: Status not OK")
			return torrentproto.Torrent{}, errors.New("Get Trackers failed")
		}

		trackernodes := make([]torrentproto.TrackerNode, len(trackers.HostPorts))
		for i, v := range trackers.HostPorts {
			trackernodes[i] = torrentproto.TrackerNode{HostPort: v}
		}
	} else {
		trackernodes := make([]torrentproto.TrackerNode, 1)
		trackernodes[0] := torrentproto.TrackerNode{HostPort: "this is my port"}
	}

	chunkhashes := make(map[int]string)
	for i := 0; i < numChunks; i++ {
		chunkhashes[i] = "banana" // weird coincidence that all chunks hash to the same value.
	}

	torrent := torrentproto.Torrent{
		ID: torrentproto.ID{Name: "TestName", Hash: "TestHash"},
		ChunkHashes: chunkhashes,
		TrackerNodes: trackernodes,
		ChunkSize: 10,
		FileSize: 10 * numChunks}
	return torrent, nil
}

// test GetTrackers for a single node
func getTrackersTestOneNode() bool {
	tester, err := createTracker("", 1, 9090, 0)
	if err != nil {
		LOGE.Println("Error creating tracker")
		return false
	}
	trackers, err := tester.GetTrackers()
	if err != nil {
		LOGE.Println("Error getting trackers")
		return false
	}
	if trackers.Status != trackerproto.OK {
		LOGE.Println("Get Trackers: Status not OK")
		return false
	}
	LOGE.Println("Trackers Found:")
	for _, v := range trackers.HostPorts {
		LOGE.Println(v)
	}
	return true
}

// testGetTrackers for multiple nodes
func getTrackersTestThreeNodes() bool {
	master := net.JoinHostPort("localhost", strconv.Itoa(9090))
	tester1, err1 := createTracker("", 3, 9090, 0)
	tester2, err2 := createTracker(master, 3, 8080, 1)
	tester3, err3 := createTracker(master, 3, 7070, 2)
	if err1 != nil || err2 != nil || err3 != nil {
		LOGE.Println("Error creating trackers")
		return false
	}

	trackers, err := tester1.GetTrackers()
	if err != nil {
		LOGE.Println("Error getting trackers")
		return false
	}
	if trackers.Status != trackerproto.OK {
		LOGE.Println("Get Trackers: Status not OK")
		return false
	}
	LOGE.Println("Trackers Found:")
	for _, v := range trackers.HostPorts {
		LOGE.Println(v)
	}
	return true
}

// test CreateEntry with single node
func createEntryTestOneNode() bool {
	tester, err := createTracker("", 1, 9090, 0)
	if err != nil {
		LOGE.Println("Error creating tracker")
		return false
	}

	torrent, err := newTorrentInfo(tester, true, 3)
	if err != nil {
		LOGE.Println("Could not create torrent")
		return false
	}

	// Test that we can add a torrent
	reply, err := tester.CreateEntry(torrent)
	if reply.Status != trackerproto.OK {
		LOGE.Println("Create Entry: Status not OK")
		return false
	}

	// Test that we can't add the same torrent twice
	reply, err = tester.CreateEntry(torrent)
	if reply.Status != trackerproto.InvalidID {
		LOGE.Println("Create Entry: Status not InvalidID"}
		return false
	}

	// Test a torrent with the wrong trackers
	badtorrent, err := newTorrentInfo(tester, false, 5)
	if err != nil {
		LOGE.Println("Could not create torrent")
		return false
	}
	reply, err = tester.CreateEntry(torrent)
	if reply.Status != trackerproto.InvalidTrackers {
		LOGE.Println("Create Entry: Status not InvalidTrackers")
		return false
	}
	return true
}

// test CreateEntry on three nodes
func createEntryTestThreeNodes() bool {
	master := net.JoinHostPort("localhost", strconv.Itoa(9090))
	tester1, err1 := createTracker("", 3, 9090, 0)
	tester2, err2 := createTracker(master, 3, 8080, 1)
	tester3, err3 := createTracker(master, 3, 7070, 2)
	if err1 != nil || err2 != nil || err3 != nil {
		LOGE.Println("Error creating trackers")
		return false
	}

	torrent, err := newTorrentInfo(tester, true, 3)
	if err != nil {
		LOGE.Println("Could not create torrent")
		return false
	}

	// Test that we can add a torrent
	reply, err := tester1.CreateEntry(torrent)
	if reply.Status != trackerproto.OK {
		LOGE.Println("Create Entry: Status not OK")
		return false
	}

	// Test that we can't add the same torrent twice
	reply, err = tester1.CreateEntry(torrent)
	if reply.Status != trackerproto.InvalidID {
		LOGE.Println("Create Entry: Status not InvalidID"}
		return false
	}

	// Test that we can't add the same torrent a second time on a different tracker
	// Essentially tests that we have the same data across the trackers
	reply, err = tester2.CreateEntry(torrent)
	if reply.Status != trackerproto.InvalidID {
		LOGE.Println("Create Entry: Status not InvalidID"}
		return false
	}

	// Test a torrent with the wrong trackers
	badtorrent, err := newTorrentInfo(tester, false, 5)
	if err != nil {
		LOGE.Println("Could not create torrent")
		return false
	}
	reply, err = tester1.CreateEntry(torrent)
	if reply.Status != trackerproto.InvalidTrackers {
		LOGE.Println("Create Entry: Status not InvalidTrackers")
		return false
	}
	return true
}

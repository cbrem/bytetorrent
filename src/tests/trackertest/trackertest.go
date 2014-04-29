package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"rand"
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

func createCluster(numNodes int) ([](*trackerTester), error) {
	basePort := 9091 + 100*(rand.Int() % 100)
	cluster := make([](*trackerTester), numNodes)
	doneChan := make(chan (*trackerTester))
	master := net.JoinHostPort("localhost", strconv.Itoa(basePort))

	// Spawn the master tracker
	go func() {
		tester, err := createTracker("", numNodes, basePort, 0)
		if err != nil {
			LOGE.Println("Could not create tracker")
			doneChan <- nil
		} else {
			doneChan <- tester
		}
	} ()

	// Spawn the rest of the trackers in the ring
	for i := 1; i < numNodes; i++ {
		go func (doneChan chan *trackerTester) {
			tester, err := createTracker(master, numNodes, basePort+17*i, i)
			if err != nil {
				LOGE.Println("Could not create tracker")
				doneChan <- nil
			} else {
				doneChan <- tester
			}
		} ()
	}

	// Now wait for the nodes to respond
	bad := false
	i := 0
	for i < numNodes {
		cluster[i] = <-doneChan
		if cluster[i] == nil {
			bad = true
		}
	}
	if bad {
		return cluster, errors.New("Could not create cluster")
	} else {
		return cluster, nil
	}
}

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
	cluster, err := createCluster(3)
	if err != nil {
		LOGE.Println("Error creating cluster")
		return false
	}

	trackers, err := cluster[0].GetTrackers()
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
	cluster, err := createCluster(3)
	if err != nil {
		LOGE.Println("Error creating cluster")
		return false
	}

	torrent, err := newTorrentInfo(cluster[0], true, 3)
	if err != nil {
		LOGE.Println("Could not create torrent")
		return false
	}

	// Test that we can add a torrent
	reply, err := cluster[0].CreateEntry(torrent)
	if reply.Status != trackerproto.OK {
		LOGE.Println("Create Entry: Status not OK")
		return false
	}

	// Test that we can't add the same torrent twice
	reply, err = cluster[0].CreateEntry(torrent)
	if reply.Status != trackerproto.InvalidID {
		LOGE.Println("Create Entry: Status not InvalidID"}
		return false
	}

	// Test that we can't add the same torrent a second time on a different tracker
	// Essentially tests that we have the same data across the trackers
	reply, err = cluster[1].CreateEntry(torrent)
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
	reply, err = cluster[0].CreateEntry(torrent)
	if reply.Status != trackerproto.InvalidTrackers {
		LOGE.Println("Create Entry: Status not InvalidTrackers")
		return false
	}
	return true
}

// Simple test
// Add two "peers" for the same chunk, then remove one
func testCluster(numNodes int) bool {
	cluster, err := createCluster(numNodes)
	if err != nil {
		LOGE.Println("Error creating tracker")
		return false
	}

	torrent, err := newTorrentInfo(cluster[0], true, 3)
	if err != nil {
		LOGE.Println("Could not create torrent")
		return false
	}

	reply, err := cluster[0].CreateEntry(torrent)
	if reply.Status != trackerproto.OK {
		LOGE.Println("Create Entry: Status not OK")
		return false
	}

	chunk := torrentProto.ChunkID{ID: torrent.ID, 0}
	reply, err = cluster[0].ConfirmChunk(chunk, "banana")
	if err != nil {
		LOGE.Println("Error confirming chunk")
		return false
	}
	if reply.Status != trackerproto.OK {
		LOGE.Println("Confirm Chunk: Status not OK")
		return false
	}

	reply, err = cluster[0].ConfirmChunk(chunk, "apple")
	if err != nil {
		LOGE.Println("Error confirming chunk")
		return false
	}
	if reply.Status != trackerproto.OK {
		LOGE.Println("Confirm Chunk: Status not OK")
		return false
	}

	reply, err = cluster[0].ReportMissing(chunk, "banana")
	if err != nil {
		LOGE.Println("Error confirming chunk")
		return false
	}
	if reply.Status != trackerproto.OK {
		LOGE.Println("Confirm Chunk: Status not OK")
		return false
	}

	reqReply, err := cluster[0].RequestChunk(chunk)
	if err != nil {
		LOGE.Println("Error confirming chunk")
		return false
	}
	if reqReply.Status != trackerproto.OK {
		LOGE.Println("Request Chunk: Status not OK")
		return false
	}
	// Should just contain "apple"
	if len(reqReply.peers) != 1 {
		LOGE.Println("Wrong number of peers")
		return false
	} else {
		if reqReply.peers[0] != "apple" {
			LOGE.Println("Wrong Peers: " + reqReply.peers[0])
			return false
		} else {
			return true
		}
	}
}

// Tests that a 3 node cluster can still operate when one node is closed.
func testClosed() {
	cluster, err := createCluster(3)
	if err != nil {
		LOGE.Println("Could not create cluster")
		return false
	}

	// Close one of the nodes
	cluster[2].Tracker.DebugStall(0)

	// Now attempt to do something.
	torrent, err := newTorrentInfo(cluster[0], true, 3)
	if err != nil {
		LOGE.Println("Could not create torrent")
		return false
	}

	reply, err := cluster[0].CreateEntry(torrent)
	if reply.Status != trackerproto.OK {
		LOGE.Println("Create Entry: Status not OK")
		return false
	}

	chunk := torrentProto.ChunkID{ID: torrent.ID, 0}
	reply, err = cluster[0].ConfirmChunk(chunk, "banana")
	if err != nil {
		LOGE.Println("Error confirming chunk")
		return false
	}
	if reply.Status != trackerproto.OK {
		LOGE.Println("Confirm Chunk: Status not OK")
		return false
	}

	return true
}

// Tests that a 3 node cluster will NOT operate when two nodes are closed
func testClosedTwo() {
	cluster, err := createCluster(3)
	if err != nil {
		LOGE.Println("Colud not create cluster")
		return false
	}

	// Close two nodes
	cluster[1].Tracker.DebugStall(0)
	cluster[2].Tracker.DebugStall(0)

	boolChan := make(chan bool)
	T := time.AfterFunc(time.Second * time.Duration(15), func () { boolChan <- true })

	go func(cluster []*trackerTester) {
		// Now attempt to do something.
		torrent, err := newTorrentInfo(cluster[0], true, 3)
		if err != nil {
			LOGE.Println("Could not create torrent")
			boolChan <- false
		}

		reply, err := cluster[0].CreateEntry(torrent)
		if reply.Status != trackerproto.OK {
			LOGE.Println("Create Entry: Status not OK")
			boolChan <- false
		}
		boolChan <- false
	} (cluster)

	return <-boolChan
}

// Stall one node, then do stuff
// See if the stalled node can catch-up
func testStalled() {
	cluster, err := createCluster(3)
	if err != nil {
		LOGE.Println("Could not create cluster")
	}

	// Stall for 15 seconds
	cluster[2].Tracker.DebugStall(15)

	// Now attempt to do something.
	torrent, err := newTorrentInfo(cluster[0], true, 3)
	if err != nil {
		LOGE.Println("Could not create torrent")
		return false
	}

	reply, err := cluster[0].CreateEntry(torrent)
	if reply.Status != trackerproto.OK {
		LOGE.Println("Create Entry: Status not OK")
		return false
	}

	chunk := torrentProto.ChunkID{ID: torrent.ID, 0}
	reply, err = cluster[0].ConfirmChunk(chunk, "banana")
	if err != nil {
		LOGE.Println("Error confirming chunk")
		return false
	}
	if reply.Status != trackerproto.OK {
		LOGE.Println("Confirm Chunk: Status not OK")
		return false
	}

	reply, err = cluster[2].ConfirmChunk(chunk, "apple")
	if err != nil {
		LOGE.Println("Error confirming chunk")
		return false
	}
	if reply.Status != trackerproto.OK {
		LOGE.Println("Confirm Chunk: Status not OK")
		return false
	}
	return true
}

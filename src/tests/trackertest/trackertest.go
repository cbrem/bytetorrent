package main

import (
	"errors"
	"log"
	"math/rand"
	"io"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
	"torrent/torrentproto"
	"tracker"
	"tracker/trackerproto"
)

type trackerTester struct {
	cmd *exec.Cmd
	srv *rpc.Client
	in io.WriteCloser
}

var LOGE = log.New(os.Stderr, "", log.Lshortfile|log.Lmicroseconds)

func createCluster(numNodes int) ([](*trackerTester), error) {
	if numNodes <= 0 {
		return nil, errors.New("numNodes <= 0")
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	basePort := 9091 + 41*(r.Int() % 300)
	cluster := make([](*trackerTester), numNodes)
	doneChan := make(chan *trackerTester)
	master := net.JoinHostPort("localhost", strconv.Itoa(basePort))
	LOGE.Println("Master HostPort: ", master)

	gobin := os.Getenv("GOBIN")
	if gobin == "" {
		return cluster, errors.New("GOBIN not set")
	}

	go func () {
		// Start the master server
		masterCmd := exec.Command(filepath.Join(gobin, "tracker_runner"), strconv.Itoa(basePort), strconv.Itoa(numNodes), strconv.Itoa(0))
		in, err := masterCmd.StdinPipe()

		masterCmd.Start()
		srv, err := rpc.DialHTTP("tcp", master)
		for err != nil {
			srv, err = rpc.DialHTTP("tcp", master)
		}
		doneChan <- &trackerTester{
			cmd: masterCmd,
			srv: srv,
			in: in}
	} ()

	// Spawn the non-master trackers in the cluster
	for i := 1; i < numNodes; i++ {
		go func (id int) {
			port := basePort + 17*id
			trackcmd := exec.Command(filepath.Join(gobin, "tracker_runner"), strconv.Itoa(port),
					strconv.Itoa(numNodes), strconv.Itoa(id), master)
			in, err := trackcmd.StdinPipe()

			trackcmd.Start()
			srv, err := rpc.DialHTTP("tcp", net.JoinHostPort("localhost", strconv.Itoa(port)))
			for err != nil {
				srv, err = rpc.DialHTTP("tcp", net.JoinHostPort("localhost", strconv.Itoa(port)))
			}
			doneChan <- &trackerTester{
				cmd: trackcmd,
				srv: srv,
				in: in}
		} (i)
	}

	resp := 0
	for resp < numNodes {
		cluster[resp] = <-doneChan
		resp++
	}
	LOGE.Println("Created Cluster")

	return cluster, nil
}

func closeCluster(cluster [](*trackerTester)) {
	for _, tracker := range cluster {
		tracker.in.Close()
		tracker.cmd.Process.Kill()
	}
}

func createTracker(master string, numNodes, port, nodeID int) (*trackerTester, error) {
	_, err := tracker.NewTrackerServer(master, numNodes, port, nodeID)
	if err != nil {
		LOGE.Println(err.Error())
		return nil, err
	}

	srv, err := rpc.DialHTTP("tcp", net.JoinHostPort("localhost", strconv.Itoa(port)))
	if err != nil {
		LOGE.Println("Could not connect to tracker")
		return nil, err
	}

	return &trackerTester{srv: srv}, nil
}

func (t *trackerTester) GetOp(seqNum int) (*trackerproto.GetReply, error) {
	args := &trackerproto.GetArgs{SeqNum: seqNum}
	reply := &trackerproto.GetReply{}
	err := t.srv.Call("PaxosTracker.GetOp", args, reply)
	return reply, err
}

func (t *trackerTester) ConfirmChunk(chunk torrentproto.ChunkID, hostPort string) (*trackerproto.UpdateReply, error) {
	args := &trackerproto.ConfirmArgs{
		Chunk: chunk,
		HostPort: hostPort}
	reply := &trackerproto.UpdateReply{}
	err := t.srv.Call("RemoteTracker.ConfirmChunk", args, reply)
	return reply, err
}

func (t *trackerTester) RequestChunk(chunk torrentproto.ChunkID) (*trackerproto.RequestReply, error) {
	args := &trackerproto.RequestArgs{Chunk: chunk}
	reply := &trackerproto.RequestReply{}
	err := t.srv.Call("RemoteTracker.RequestChunk", args, reply)
	return reply, err
}

func (t *trackerTester) CreateEntry(torrent torrentproto.Torrent) (*trackerproto.UpdateReply, error) {
	args := &trackerproto.CreateArgs{Torrent: torrent}
	reply := &trackerproto.UpdateReply{}
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
	var trackernodes []torrentproto.TrackerNode
	if trackersGood {
		trackers, err := t.GetTrackers()
		if err != nil || trackers.Status != trackerproto.OK {
			return torrentproto.Torrent{}, errors.New("Get Trackers failed")
		}

		trackernodes = make([]torrentproto.TrackerNode, len(trackers.HostPorts))
		for i, v := range trackers.HostPorts {
			trackernodes[i] = torrentproto.TrackerNode{HostPort: v}
		}
	} else {
		trackernodes = make([]torrentproto.TrackerNode, 1)
		trackernodes[0] = torrentproto.TrackerNode{HostPort: "this is my port"}
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
	cluster, err := createCluster(1)
	if err != nil {
		LOGE.Println("Error creating tracker")
		closeCluster(cluster)
		return false
	}
	LOGE.Println("Getting Trackers")
	trackers, err := cluster[0].GetTrackers()
	if err != nil {
		LOGE.Println("Error getting trackers")
		LOGE.Println(err.Error())
		closeCluster(cluster)
		return false
	}
	if trackers.Status != trackerproto.OK {
		LOGE.Println("Get Trackers: Status not OK")
		closeCluster(cluster)
		return false
	}
	LOGE.Println("Trackers Found:")
	for _, v := range trackers.HostPorts {
		LOGE.Println(v)
	}
	closeCluster(cluster)
	return true
}

// testGetTrackers for multiple nodes
func getTrackersTestThreeNodes() bool {
	cluster, err := createCluster(3)
	if err != nil {
		LOGE.Println("Error creating cluster")
		LOGE.Println(err.Error())
		closeCluster(cluster)
		return false
	}

	LOGE.Println("Getting Trackers")
	trackers, err := cluster[0].GetTrackers()
	if err != nil {
		LOGE.Println("Error getting trackers")
		closeCluster(cluster)
		return false
	}
	if trackers.Status != trackerproto.OK {
		LOGE.Println("Get Trackers: Status not OK")
		closeCluster(cluster)
		return false
	}
	LOGE.Println("Trackers Found:")
	for _, v := range trackers.HostPorts {
		LOGE.Println(v)
	}

	closeCluster(cluster)
	return true
}

// test CreateEntry with single node
func createEntryTestOneNode() bool {
	cluster, err := createCluster(1)
	if err != nil {
		LOGE.Println("Error creating tracker")
		closeCluster(cluster)
		return false
	}

	torrent, err := newTorrentInfo(cluster[0], true, 3)
	if err != nil {
		LOGE.Println("Could not create torrent")
		closeCluster(cluster)
		return false
	}

	// Test that we can add a torrent
	reply, err := cluster[0].CreateEntry(torrent)
	if reply.Status != trackerproto.OK {
		LOGE.Println("Create Entry: Status not OK")
		closeCluster(cluster)
		return false
	}

	// Test that we can't add the same torrent twice
	reply, err = cluster[0].CreateEntry(torrent)
	if reply.Status != trackerproto.InvalidID {
		LOGE.Println("Create Entry: Status not InvalidID")
		closeCluster(cluster)
		return false
	}

	// Test a torrent with the wrong trackers
	badtorrent, err := newTorrentInfo(cluster[0], false, 5)
	if err != nil {
		LOGE.Println("Could not create torrent")
		closeCluster(cluster)
		return false
	}
	reply, err = cluster[0].CreateEntry(badtorrent)
	if reply.Status != trackerproto.InvalidTrackers {
		LOGE.Println("Create Entry: Status not InvalidTrackers")
		closeCluster(cluster)
		return false
	}
	closeCluster(cluster)
	return true
}

// test CreateEntry on three nodes
func createEntryTestThreeNodes() bool {
	cluster, err := createCluster(3)
	if err != nil {
		LOGE.Println("Error creating cluster")
		closeCluster(cluster)
		return false
	}

	torrent, err := newTorrentInfo(cluster[0], true, 3)
	if err != nil {
		LOGE.Println("Could not create torrent")
		closeCluster(cluster)
		return false
	}

	// Test that we can add a torrent
	reply, err := cluster[0].CreateEntry(torrent)
	if reply.Status != trackerproto.OK {
		LOGE.Println("Create Entry: Status not OK")
		closeCluster(cluster)
		return false
	}

	// Test that we can't add the same torrent twice
	reply, err = cluster[0].CreateEntry(torrent)
	if reply.Status != trackerproto.InvalidID {
		LOGE.Println("Create Entry: Status not InvalidID")
		closeCluster(cluster)
		return false
	}

	// Test that we can't add the same torrent a second time on a different tracker
	// Essentially tests that we have the same data across the trackers
	reply, err = cluster[1].CreateEntry(torrent)
	if reply.Status != trackerproto.InvalidID {
		LOGE.Println("Create Entry: Status not InvalidID")
		closeCluster(cluster)
		return false
	}

	// Test a torrent with the wrong trackers
	badtorrent, err := newTorrentInfo(cluster[0], false, 5)
	if err != nil {
		LOGE.Println("Could not create torrent")
		closeCluster(cluster)
		return false
	}
	reply, err = cluster[0].CreateEntry(badtorrent)
	if reply.Status != trackerproto.InvalidTrackers {
		LOGE.Println("Create Entry: Status not InvalidTrackers")
		closeCluster(cluster)
		return false
	}

	closeCluster(cluster)
	return true
}

// Simple test
// Add two "peers" for the same chunk, then remove one
func testCluster(numNodes int) bool {
	LOGE.Println("Creating Cluster")
	cluster, err := createCluster(numNodes)
	if err != nil {
		LOGE.Println("Error creating tracker")
		closeCluster(cluster)
		return false
	}

	LOGE.Println("Creating Torrent")
	torrent, err := newTorrentInfo(cluster[0], true, 3)
	if err != nil {
		LOGE.Println("Could not create torrent")
		closeCluster(cluster)
		return false
	}

	LOGE.Println("Sending Torrent to Tracker")
	reply, err := cluster[0].CreateEntry(torrent)
	if reply.Status != trackerproto.OK {
		LOGE.Println("Create Entry: Status not OK")
		LOGE.Println(strconv.Itoa(int(reply.Status)))
		closeCluster(cluster)
		return false
	}

	LOGE.Println("Confirm 'banana' on Tracker")
	chunk := torrentproto.ChunkID{ID: torrent.ID, ChunkNum: 0}
	reply, err = cluster[0].ConfirmChunk(chunk, "banana")
	if err != nil {
		LOGE.Println("Error confirming chunk")
		closeCluster(cluster)
		return false
	}
	if reply.Status != trackerproto.OK {
		LOGE.Println("Confirm Chunk: Status not OK")
		closeCluster(cluster)
		return false
	}

	LOGE.Println("Confirm 'apple' on Tracker")
	reply, err = cluster[0].ConfirmChunk(chunk, "apple")
	if err != nil {
		LOGE.Println("Error confirming chunk")
		closeCluster(cluster)
		return false
	}
	if reply.Status != trackerproto.OK {
		LOGE.Println("Confirm Chunk: Status not OK")
		closeCluster(cluster)
		return false
	}

	LOGE.Println("Report 'banana' on tracker")
	reply, err = cluster[0].ReportMissing(chunk, "banana")
	if err != nil {
		LOGE.Println("Error confirming chunk")
		closeCluster(cluster)
		return false
	}
	if reply.Status != trackerproto.OK {
		LOGE.Println("Confirm Chunk: Status not OK")
		closeCluster(cluster)
		return false
	}

	LOGE.Println("Getting 'peers' for chunk")
	reqReply, err := cluster[0].RequestChunk(chunk)
	if err != nil {
		LOGE.Println("Error confirming chunk")
		closeCluster(cluster)
		return false
	}
	if reqReply.Status != trackerproto.OK {
		LOGE.Println("Request Chunk: Status not OK")
		closeCluster(cluster)
		return false
	}
	// Should just contain "apple"
	if len(reqReply.Peers) != 1 {
		LOGE.Println("Wrong number of peers")
		closeCluster(cluster)
		return false
	} else {
		if reqReply.Peers[0] != "apple" {
			LOGE.Println("Wrong Peers: " + reqReply.Peers[0])
			closeCluster(cluster)
			return false
		} else {
			LOGE.Println("Correct Peers: " + reqReply.Peers[0])
			closeCluster(cluster)
			return true
		}
	}
}

func testStress(total int) bool {
	cluster, _ := createCluster(3)

	LOGE.Println("Creating Torrent")
	torrent, _ := newTorrentInfo(cluster[0], true, 3)
	cluster[0].CreateEntry(torrent)

	chunk := torrentproto.ChunkID{ID: torrent.ID, ChunkNum: 0}
	doneChan := make(chan struct {})

	LOGE.Println("Sending Messages")
	// Have two nodes make a large number of confirms and reports
	for i := 0; i < total; i++ {
		go func () {
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			conf, err := cluster[0].ConfirmChunk(chunk, strconv.Itoa(r.Int()))
			if err != nil {
				LOGE.Println(err.Error())
			}
			if conf.Status != trackerproto.OK {
				LOGE.Println("Confirm Chunk: Status not OK")
			}

			doneChan <- struct{}{}
		} ()
	}
	LOGE.Println("Waiting for things to finish")
	fin := 0
	for fin < total {
		<-doneChan
		fin++
		if fin % 50 == 0 {
			LOGE.Println("Finished: ", fin)
		}
	}
	LOGE.Println("Everything in")

	seqNum := 0
	ok := true
	matching := true
	for matching && ok {
		reply0, err0 := cluster[0].GetOp(seqNum)
		reply2, err2 := cluster[1].GetOp(seqNum)

		if err0 != nil || err2 != nil {
			LOGE.Println("Error getting operation.")
			closeCluster(cluster)
			return false
		}
		if reply0.Status == trackerproto.OutOfDate {
			ok = false
		}
		val0 := reply0.Value
		val2 := reply2.Value
		valsEq := val0.OpType == val2.OpType && val0.Chunk == val2.Chunk && val0.ClientAddr == val2.ClientAddr
		matching = matching && valsEq && (reply0.Status == reply2.Status)

		seqNum++
	}
	LOGE.Println("SeqNum: ", seqNum)
	closeCluster(cluster)
	return matching
}

// Test with dualing leaders
func testDualing(total int) bool {
	cluster, _ := createCluster(3)

	LOGE.Println("Creating Torrent")
	torrent, _ := newTorrentInfo(cluster[0], true, 3)
	cluster[0].CreateEntry(torrent)

	chunk := torrentproto.ChunkID{ID: torrent.ID, ChunkNum: 0}
	doneChan := make(chan int)

	LOGE.Println("Sending Messages")
	// Have two nodes make a large number of confirms and reports
	for i := 0; i < total; i++ {
		go func (id int) {
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			conf, err := cluster[id].ConfirmChunk(chunk, strconv.Itoa(r.Int()))
			if err != nil {
				LOGE.Println(err.Error())
			}
			if conf.Status != trackerproto.OK {
				LOGE.Println("Confirm Chunk: Status not OK")
			}

			doneChan <- id
		} (i % 2)
	}
	LOGE.Println("Waiting for things to finish")
	fin := make([]int, 2)
	fin[0] = 0
	fin[1] = 0
	for fin[0] + fin[1] < total {
		id := <-doneChan
		fin[id]++
		if (fin[0] + fin[1]) % 50 == 0 {
			LOGE.Println("Finished: ", fin[0], fin[1])
		}
	}
	LOGE.Println("Everything in")

	seqNum := 0
	ok := true
	matching := true
	for matching && ok {
		reply0, err0 := cluster[0].GetOp(seqNum)
		reply2, err2 := cluster[1].GetOp(seqNum)

		if err0 != nil || err2 != nil {
			LOGE.Println("Error getting operation.")
			closeCluster(cluster)
			return false
		}
		if reply0.Status == trackerproto.OutOfDate {
			ok = false
		}
		val0 := reply0.Value
		val2 := reply2.Value
		valsEq := val0.OpType == val2.OpType && val0.Chunk == val2.Chunk && val0.ClientAddr == val2.ClientAddr
		matching = matching && valsEq && (reply0.Status == reply2.Status)

		seqNum++
	}
	LOGE.Println("SeqNum: ", seqNum)
	closeCluster(cluster)
	return matching
}

// Tests that a 3 node cluster can still operate when one node is closed.
func testClosed() bool {
	cluster, err := createCluster(3)
	if err != nil {
		LOGE.Println("Could not create cluster")
		closeCluster(cluster)
		return false
	}

	LOGE.Println("Making Torrent")
	// Now attempt to do something.
	torrent, err := newTorrentInfo(cluster[0], true, 3)
	if err != nil {
		LOGE.Println("Could not create torrent")
		closeCluster(cluster)
		return false
	}

	LOGE.Println("Killing Process")
	// Close one of the nodes
	if err := cluster[2].cmd.Process.Signal(syscall.SIGKILL); err != nil {
		LOGE.Println("Could not close node")
		closeCluster(cluster)
		return false
	}

	boolChan := make(chan bool)
	time.AfterFunc(time.Second * time.Duration(10), func () { boolChan <- false})

	go func () {
		LOGE.Println("Sending Torrent to tracker")
		reply, err := cluster[0].CreateEntry(torrent)
		if reply.Status != trackerproto.OK {
			LOGE.Println("Create Entry: Status not OK")
			closeCluster(cluster)
			boolChan <- false
		}

		LOGE.Println("Confirming chunks")
		chunk := torrentproto.ChunkID{ID: torrent.ID, ChunkNum: 0}
		reply, err = cluster[0].ConfirmChunk(chunk, "banana")
		if err != nil {
			LOGE.Println("Error confirming chunk")
			closeCluster(cluster)
			boolChan <- false
		}
		if reply.Status != trackerproto.OK {
			LOGE.Println("Confirm Chunk: Status not OK")
			closeCluster(cluster)
			boolChan <- false
		}
		boolChan <- true
	} ()

	result := <-boolChan
	closeCluster(cluster)
	return result
}

// Tests that a 3 node cluster will NOT operate when two nodes are closed
func testClosedTwo() bool {
	cluster, err := createCluster(3)
	if err != nil {
		LOGE.Println("Could not create cluster")
		closeCluster(cluster)
		return false
	}

	LOGE.Println("Making Torrent")
	// Now attempt to do something.
	torrent, err := newTorrentInfo(cluster[0], true, 3)
	if err != nil {
		LOGE.Println("Could not create torrent")
		closeCluster(cluster)
		return false
	}

	LOGE.Println("Sending Torrent to tracker")
	reply, err := cluster[0].CreateEntry(torrent)
	if reply.Status != trackerproto.OK {
		LOGE.Println("Create Entry: Status not OK")
		closeCluster(cluster)
		return false
	}

	// Close two nodes
	LOGE.Println("Closing Nodes")
	if err := cluster[1].cmd.Process.Signal(syscall.SIGKILL); err != nil {
		LOGE.Println("Could not close node 1")
		closeCluster(cluster)
		return false
	}
	if err := cluster[2].cmd.Process.Signal(syscall.SIGKILL); err != nil {
		LOGE.Println("Could not close node 2")
		closeCluster(cluster)
		return false
	}

	boolChan := make(chan bool, 1)
	time.AfterFunc(time.Second * time.Duration(10), func () { boolChan <- true })

	LOGE.Println("Attempting to write")
	go func(cluster []*trackerTester) {
		chunk := torrentproto.ChunkID{ID: torrent.ID, ChunkNum: 0}
		reply, err = cluster[0].ConfirmChunk(chunk, "banana")

		if err == nil && reply.Status != trackerproto.OK {
			LOGE.Println("Create Entry: Status not OK")
			LOGE.Println(reply.Status)
		}
		boolChan <- false
	} (cluster)

	passed := <-boolChan
	closeCluster(cluster)
	return passed
}

// Stall one node, then do stuff
// See if the stalled node can catch-up
func testStalled() bool {
	cluster, err := createCluster(3)
	if err != nil {
		LOGE.Println("Could not create cluster")
		closeCluster(cluster)
		return false
	}

	// Now attempt to do something.
	torrent, err := newTorrentInfo(cluster[0], true, 3)
	if err != nil {
		LOGE.Println("Could not create torrent")
		closeCluster(cluster)
		return false
	}

	reply, err := cluster[0].CreateEntry(torrent)
	if reply.Status != trackerproto.OK {
		LOGE.Println("Create Entry: Status not OK")
		closeCluster(cluster)
		return false
	}

	chunk := torrentproto.ChunkID{ID: torrent.ID, ChunkNum: 0}

	// Stall for 5 seconds
	LOGE.Println("Stalling tracker")
	cluster[2].cmd.Process.Signal(syscall.SIGSTOP)
	/*
	if _, err := fmt.Fprintln(cluster[2].in, "5"); err != nil {
		LOGE.Println("Could not stall node")
		closeCluster(cluster)
		return false
	}
	*/

	doneChan := make(chan struct{})

	LOGE.Println("Sending Messages")
	for i := 0; i < 50; i++ {
		go func () {
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			conf, err := cluster[0].ConfirmChunk(chunk, strconv.Itoa(r.Int()))
			if err != nil {
				LOGE.Println(err.Error())
			}
			if conf.Status != trackerproto.OK {
				LOGE.Println("Confirm Chunk: Status not OK")
			}
			doneChan <- struct{}{}
		} ()
	}

	LOGE.Println("Waiting for messages to finish")
	fin := 0
	for fin < 50 {
		<-doneChan
		fin++
	}

	// Call something on the stalled tracker
	// The call will block until the tracker comes back online
	LOGE.Println("Restarting Tracker")
	cluster[2].cmd.Process.Signal(syscall.SIGCONT)
	LOGE.Println("Call on stalled tracker")
	reply, err = cluster[2].ConfirmChunk(chunk, "apple")
	if err != nil {
		LOGE.Println("Error confirming chunk")
		closeCluster(cluster)
		return false
	}
	if reply.Status != trackerproto.OK {
		LOGE.Println("Confirm Chunk: Status not OK")
		LOGE.Println(reply.Status)
		closeCluster(cluster)
		return false
	}

	LOGE.Println("Verifying logs")
	seqNum := 0
	ok := true
	matching := true
	for matching && ok {
		reply0, err0 := cluster[0].GetOp(seqNum)
		reply2, err2 := cluster[2].GetOp(seqNum)

		if err0 != nil || err2 != nil {
			LOGE.Println("Error getting operation.")
			closeCluster(cluster)
			return false
		}
		if reply0.Status == trackerproto.OutOfDate {
			ok = false
		}
		val0 := reply0.Value
		val2 := reply2.Value
		valsEq := val0.OpType == val2.OpType && val0.Chunk == val2.Chunk && val0.ClientAddr == val2.ClientAddr
		matching = matching && valsEq && (reply0.Status == reply2.Status)
		seqNum++
	}

	closeCluster(cluster)
	return matching
}

func main() {
	tests := 0
	pass := 0

	tests++
	LOGE.Println("----------- getTrackersTestOneNode")
	if !getTrackersTestOneNode() {
		LOGE.Println("---------------------- Failed getTrackersTestOneNode")
	} else {
		pass++
		LOGE.Println("Passed getTrackersTestOneNode")
	}

	tests++
	LOGE.Println("----------- getTrackersTestThreeNodes")
	if !getTrackersTestThreeNodes() {
		LOGE.Println("---------------------- Failed getTrackersTestThreeNodes")
	} else {
		pass++
		LOGE.Println("Passed getTrackersTestThreeNodes")
	}

	tests++
	LOGE.Println("----------- createEntryTestOneNode")
	if !createEntryTestOneNode() {
		LOGE.Println("---------------------- Failed createEntryTestOneNode")
	} else {
		pass++
		LOGE.Println("Passed createEntryTestOneNode")
	}

	tests++
	LOGE.Println("----------- createEntryTestThreeNodes")
	if !createEntryTestThreeNodes() {
		LOGE.Println("---------------------- Failed createEntryTestThreeNodes")
	} else {
		pass++
		LOGE.Println("Passed createEntryTestThreeNodes")
	}

	tests++
	LOGE.Println("----------- testCluster one node")
	if !testCluster(1) {
		LOGE.Println("---------------------- Failed testCluster one node")
	} else {
		pass++
		LOGE.Println("Passed testCluster one node")
	}

	tests++
	LOGE.Println("----------- testCluster three nodes")
	if !testCluster(3) {
		LOGE.Println("---------------------- Failed testCluster three nodes")
	} else {
		pass++
		LOGE.Println("Passed testCluster three nodes")
	}

	tests++
	LOGE.Println("----------- testStress")
	if !testStress(100) {
		LOGE.Println("---------------------- Failed testStress")
	} else {
		pass++
		LOGE.Println("Passed testStress")
	}

	tests++
	LOGE.Println("----------- testDualing")
	if !testDualing(500) {
		LOGE.Println("---------------------- Failed testDualing")
	} else {
		pass++
		LOGE.Println("Passed testDualing")
	}

	tests++
	LOGE.Println("----------- testClosed")
	if !testClosed() {
		LOGE.Println("---------------------- Failed testClosed")
	} else {
		pass++
		LOGE.Println("Passed testClosed")
	}

	tests++
	LOGE.Println("----------- testClosedTwo")
	if !testClosedTwo() {
		LOGE.Println("---------------------- Failed testClosedTwo")
	} else {
		pass++
		LOGE.Println("Passed testClosedTwo")
	}

	tests++
	LOGE.Println("----------- testStalled")
	if !testStalled() {
		LOGE.Println("---------------------- Failed testStalled")
	} else {
		pass++
		LOGE.Println("Passed testStalled")
	}
	LOGE.Println("Passed: ", pass, "/", tests)
}

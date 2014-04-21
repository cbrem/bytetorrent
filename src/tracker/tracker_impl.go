package tracker

import (
	"container/list"
	"errors"
	"net"
	"net/http"
	"net/rpc"
	"rpc/trackerrpc"
	"runtime"
)

// The time between RegisterServer calls from a slave server, in seconds
const REGISTER_PERIOD = 1

type PaxosType int

const (
	Prepare PaxosType = iota
	Accept
	Commit
)

type Register struct {
	Args  *trackerrpc.RegisterArgs
	Reply chan *trackerrpc.RegisterReply
}

type Get struct {
	Args *trackerrpc.GetArgs
	Reply chan *trackerrpc.GetReply
}

type Prepare struct {
	Args  *trackerrpc.PrepareArgs
	Reply chan *trackerrpc.PrepareReply
}

type Accept struct {
	Args *trackerrpc.AcceptArgs
	Reply chan *trackerrpc.AcceptReply
}

type Commit struct {
	Args *trackerrpc.CommitArgs
	Reply chan *trackerrpc.CommitReply
}

type Request struct {
	Args *trackerrpc.RequestArgs
	Reply chan *trackerrpc.RequestReply
}

type Confirm struct {
	Args *trackerrpc.ConfirmArgs
	Reply chan *trackerrpc.ConfirmArgs
}

type Report struct {
	Args *trackerrpc.ReportArgs
	Reply chan *trackerrpc.ReportArgs
}

type PaxosReply struct {
	Status    trackerrpc.Status
	ReqPaxNum int
	PaxNum    int
	Value     trackerrpc.Operation
	SeqNum    int
}

type PaxosBroadcast struct {
	Type  PaxosType
	Value trackerrpc.Operation
	Reply chan *PaxosReply
}

type PaxosDone struct {
	Quorum int
	AccV   trackerrpc.Operation
}

type trackerServer struct {
	// Cluster Set-up
	nodes                []trackerrpc.Node
	numNodes             int
	masterServerHostPort string
	port                 int
	registers            chan *Register
	nodeID               int
	trackers             []*rpc.Client
	paxCom               [](chan *PaxosBroadcast)

	// Channels for rpc calls
	prepares             chan *Prepare
	accepts              chan *Accept
	commits              chan *Commit
	gets                 chan *Get
	requests             chan *Request
	confirms             chan *Confirm
	reports              chan *Report
	pending              chan *trackerrpc.Operation
	outOfDate            chan int

	// Paxos Stuff
	myN                  int
	highestN             int
        accN                 int
        accV                 trackerrpc.Operation

	// Sequencing / Logging
	seqNum               int
	log                  map[int]trackerrpc.Operation

	// Actual data storage
	numChunks            map[string]int                     // Maps torrentID -> number of chunks
	peers                map[string](map[string](struct{})) // Maps torrentID:chunkNum -> list of host:port with that chunk
}

func NewTrackerServer(masterServerAddr string, numNodes, port, nodeID int) (TrackerServer, error) {
	t := &trackerServer {
		masterServerHostPort: masterServerAddr,
		nodeID:               nodeID,
		nodes:                nil,
		numNodes:             numNodes,
		port:                 port,
		accepts:              make(chan *Accept),
		commits:              make(chan *Commit),
		confirms:             make(chan *Confirm),
		gets:                 make(chan *Get),
		prepares:             make(chan *Prepare),
		registers:            make(chan *Register),
		reports:              make(chan *Report),
		requests:             make(chan *Request),
		myN:                  nodeID,
                highestN:             0
		accV:                 trackerrpc.Operation{OpType: trackerrpc.None}
		seqNum:               0
		log:                  make(map[int]trackerrpc.Opeartion)
		numChunks:            make(map[string]int)
		peers:                make(map[string](map[string](struct{})))
		trackers:             make([]*rpc.Client, numNodes)
		outOfDate             make(chan int)
		}

	// Attempt to service connections on the given port.
	ln, lnErr := net.Listen("tcp", net.JoinHostPort("localhost", strconv.Itoa(port)))
	if lnErr != nil {
		return nil, lnErr
	}

	// Configure this TrackerServer to receive RPCs over HTTP on a
	// trackerrpc.Tracker interface.
	if regErr := rpc.Register(trackerrpc.WrapTracker(t)); regErr != nil {
		return nil, regErr
	}

	// New configure this TrackerServer to receive RPCs over HTTP on a
	// trackerrpc.Paxos interface
	if regErr := rpc.Register(trackerrpc.WrapPaxos(t)); regErr != nil {
		return nil, regErr
	}

	rpc.HandleHTTP()
	go http.Serve(ln, nil)

	// Wait for all TrackerServers to join the ring.
	var joinErr error
	if masterServerHostPort == "" {
		// This is the master StorageServer.
		joinErr = t.masterAwaitJoin()

		// Yield to reduce the chance that the master returns before the slaves.
		runtime.Gosched()
	} else {
		// This is a slave TrackerServer.
		joinErr = t.slaveAwaitJoin()
	}

	if joinErr != nil {
		return nil, joinErr
	} else {
		// We've registered with the master, and gotten a list of all servers.
		// We need to connect to all of them over rpc,
		// then add these data points to t.trackers
		for _, node := range t.nodes {
			trackerRPC, err := rpc.DialHTTP("tcp", node.HostPort)
			if err != nil {
				return nil, err
			}
			t.trackers[node.NodeID] = trackerRPC
		}
	}

	// Start this TrackerServer's eventHandler, which will respond to RPCs,
	// and return it.
	go t.eventHandler()

	// Spawn a goroutine to talk to the other Paxos Nodes
	go t.paxosHandler()

	return t, nil
}

func (t *trackerServer) RegisterServer(args *trackerrpc.RegisterArgs, reply *trackerrpc.RegisterReply) error {
	replyChan := make(chan *trackerrpc.RegisterReply)
	register := &Register{
		Args:  args,
		Reply: replyChan}
	t.registers <- register
	*reply = *(<-replyChan)
	return nil
}

func (t *trackerServer) GetOp(args *trackerrpc.GetArgs, reply *trackerrpc.GetReply) error {
	replyChan := make(chan *trackerrpc.GetOp)
	get := &Get{
		Args:  args,
		Reply: replyChan}
	t.gets <- get
	*reply = *(<-replyChan)
	return nil
}

func (t *trackerServer) Prepare(*PrepareArgs, *PrepareReply) error {
	replyChan := make(chan *trackerrpc.PrepareReply)
	prepare := &Prepare{
		Args:  args,
		Reply: replyChan}
	t.prepares <- prepare
	*reply = *(<-replyChan)
	return nil
}

func (t *trackerServer) Accept(*AcceptArgs, *AcceptReply) error {
	replyChan := make(chan *trackerrpc.AcceptReply)
	accept := &Accept{
		Args:  args,
		Reply: replyChan}
	t.accepts <- accept
	*reply = *(<-replyChan)
	return nil
}

func (t *trackerServer) Commit(*CommitArgs, *CommitReply) error {
	replyChan := make(chan *trackerrpc.CommitReply)
	commit := &Commit{
		Args:  args,
		Reply: replyChan}
	t.commits <- commit
	*reply = *(<-replyChan)
	return nil
}

func (t *trackerServer) ReportMissing(*ReportArgs, *ReportReply) error {
	replyChan := make(chan *trackerrpc.ReportReply)
	report := &Report{
		Args:  args,
		Reply: replyChan}
	t.reports <- report
	*reply = *(<-replyChan)
	return nil
}

func (t *trackerServer) ConfirmChunk(*ConfirmArgs, *ConfirmReply) error {
	replyChan := make(chan *trackerrpc.ConfirmReply)
	confirm := &Confirm{
		Args:  args,
		Reply: replyChan}
	t.confirms <- confirm
	*reply = *(<-replyChan)
	return nil
}

func (t *trackerServer) RequestChunk(*RequestArgs, *RequestReply) error {
	replyChan := make(chan *trackerrpc.RequestReply)
	request := &Request{
		Args:  args,
		Reply: replyChan}
	t.requests <- request
	*reply = *(<-replyChan)
	return nil
}

// Waits for all slave trackerServers to call the master's RegisterServer RPC.
func (t *trackerServer) masterAwaitJoin() error {
	// Initialize the array of Nodes, and create a map of all slaves that have
	// registered (and another of those who have received an OK) for fast
	// lookup.
	nodeIDs := make(map[int]struct{})
	okIDs := make(map[int]struct{})
	t.nodes = make([]trackerrpc.Node, 0)

	// Count the master server.
	//
	// NOTE: We always list the host as localhost.
	nodeIDs[t.nodeID] = struct{}{}
	okIDs[t.nodeID] = struct{}{}
	t.nodes = append(t.nodes, trackerrpc.Node{
		HostPort: net.JoinHostPort("localhost", strconv.Itoa(t.port)),
		NodeID:   t.nodeID})

	// Loop until we've heard from (and replied to) all nodes.
	for len(okIDs) != t.numNodes {
		// A node wants to register.
		register := <-t.registers:
		node := register.Args.TrackerInfo
		if _, ok := nodeIDs[node.NodeID]; !ok {
			// This is a new nodeId.
			nodeIDs[node.NodeID] = struct{}{}
			t.nodes = append(t.nodes, node)
		}

		// Determine the status to return.
		var status trackerrpc.Status
		if len(t.nodes) == t.numNodes {
			status = trackerrpc.OK

			// If this node will receive an OK, record this.
			okIDs[node.NodeID] = struct{}{}
		} else {
			status = trackerrpc.NotReady
		}

		// Reply to the node.
		nodes := make([]trackerrpc.Node, len(t.nodes))
		copy(nodes, t.nodes)
		register.Reply <- &trackerrpc.RegisterReply{
			Status:  status,
			Servers: nodes}
	}

	// We've heard from all nodes.
	return nil
}

// Waits for the master storageServer to accept a slave's RegisterServer RPC
// and confirm that all other slaves have joined.
func (t *trackerServer) slaveAwaitJoin() error {
	// Connect to the master trackerServer, retrying until we succeed.
	var conn *rpc.Client
	for conn == nil {
		if conn, _ = rpc.DialHTTP("tcp", t.masterServerHostPort); conn == nil {
			// Sleep, and try again later.
			time.Sleep(time.Second * time.Duration(REGISTER_PERIOD))
		}
	}

	// Make RegisterServer RPCs to the master trackerServer until it confirms
	// that the ring is complete.
	args := &trackerrpc.RegisterArgs{
		ServerInfo: trackerrpc.Node{
			HostPort: net.JoinHostPort("localhost", strconv.Itoa(t.port)),
			NodeID:   t.nodeID}}
	reply := &storagerpc.RegisterReply{}

	for {
		if callErr := conn.Call("Paxos.RegisterServer", args, reply); callErr != nil {
			// Failed to make RPC to storageServer.
			return callErr
		}

		if reply.Status == trackerrpc.OK {
			// The ring is ready.
			break
		}

		// Wait for a set period before trying again.
		time.Sleep(time.Second * time.Duration(REGISTER_PERIOD))
	}

	// Record which nodes are in the ring, and return.
	t.nodes = make([]trackerrpc.Node, len(reply.Servers))
	copy(t.nodes, reply.Servers)
	return nil
}

func (t *trackerServer) eventHandler() {
	for {
		select {
		case prep := <-t.prepares:
			reply := trackerrpc.PrepareReply{
				PaxNum: accN,
				Value:  accV}
			if prep.Args.SeqNum != t.seqNum {
				if prep.Args.SeqNum < t.seqNum {
					reply.Status = trackerrpc.OutOfDate
				} else {
					// This tracker is out of date
					// It should not be validating updates
					reply.Status = trackerrpc.Reject
					prep.Reply <- &reply
					t.catchUp(prep.Args.SeqNum)
				}
			} else if prep.Args.PaxNum < t.highestN {
				reply.Status = trackerrpc.Reject
				prep.Reply <- &reply
			} else {
				reply.Status = trackerrpc.OK
				prep.Reply <- &reply
			}
		case acc := <-t.accepts:
			if acc.Args.SeqNum < t.seqNum {
				status = trackerrpc.OutOfDate
			} else if acc.Args.PaxNum < t.highestN {
				status = trackerrpc.Reject
			} else {
				status = trackerrpc.OK
				t.accN = acc.Args.PaxNum
				t.accV = acc.Args.Value
			}
			acc.Reply <- &trackerrpc.AcceptReply{Status: status}
		case com := <-t.commits:
			v := com.Args.Value
			t.commitOp(v)
			com.Reply <- &trackerrpc.CommitReply{}
		case get := <-t.gets:
			s := gets.Args.SeqNum
			if s > t.seqNum {
				get.Reply <- &trackerrpc.GetReply{Status: trackerrpc.OutOfDate}
			} else {
				get.Reply <- &trackerrpc.GetReply{
					Status: trackerrpc.OK,
					Value:  t.log[s]}
				t.catchUp(s)
			}
		case rep := <-t.reports:
		case conf := <-t.confirms:
			numChunks, ok := t.numChunks[req.Args.ID]
			if !ok {
				req.Reply <- &trackerrpc.RequestReply{Status: trackerrpc.FileNotfound}
			} else if req.Args.ChunkNum < 0 || req.Args.ChunkNum >= numChunks {
				req.Reply <- &trackerrpc.RequestReply{Status: trackerrpc.OutOfRange}
			} else {
				op := &trackerrpc.Operation{
					OpType: trackerrpc.Add,
					ID: conf.Args.ID,
					ChunkNum: conf.Args.ChunkNum,
					ClientAddr: conf.Args.Addr}
				t.pending <- op
			}
		case req := <-t.requests:
			numChunks, ok := t.numChunks[req.Args.ID]
			if !ok {
				req.Reply <- &trackerrpc.RequestReply{Status: trackerrpc.FileNotfound}
			} else if req.Args.ChunkNum < 0 || req.Args.ChunkNum >= numChunks {
				req.Reply <- &trackerrpc.RequestReply{Status: trackerrpc.OutOfRange}
			} else {
				key := req.Args.ID + ":" + strconv.Itoa(req.Args.ChunkNum)
				peers := make([]string, 0)
				for k, _ := range t.peers[key] {
					append(peers, k)
				}
				req.Reply <- &trackerrpc.RequestReply{
					Status: trackerrpc.OK,
					Peers: peers}
			}
		}
	}
}

// t commits the operation to memory, and logs it
func (t *trackerServer) commitOp(v trackerrpc.Operation) {
	// Log the change
	t.log[t.seqNum] = v
	t.seqNum++
	t.accN = 0
	t.accV = tracker.Opp{OpType: trackerrpc.None}

	// Now make the change
	key := v.ID + ":" + strconv.Itoa(v.ChunkNum)
	m, ok := peers[key]
	if !ok {
		peers[key] = make(map[string](struct{}))
		m = peers[key]
	}

	if v.OpType == trackerrpc.Add {
		m[v.ClientAddr] = struct{}{}
	} else if v.OpType == trackerrpc.Remove {
		delete(m,v.ClientAddr)
	}
}

// t contacts other servers in an attempt to catch-up
// with missed changes
func (t *trackerServer) catchUp(target int) {
	loops := 0
	current := (t.nodeID + 1) % t.numNodes
	for t.seqNum < target {
		args := &trackerrpc.GetArgs{SeqNum: t.seqNum}
		reply := &trackerrpc.GetReply{}
		if err := t.trackers[current].Call("Paxos.GetOp", args, reply); err != nil {
			// there was an issue, so let's try another server
			current = (current + 1) % t.numNodes
			// If we've looped around the entire way and we're not done,
			// then delay before bothering the other trackers again
			if current == t.nodeID {
				loops++
				current = (current + 1) % t.numNodes
				time.Sleep(time.Second * time.Duration(loops * REGISTER_PERIOD))
			}
		} else {
			if reply.Status == trackerrpc.OK {
				// This increments t.seqNum
				t.commitOp(reply.Value)
			} else {
				// Server didn't have operation, so let's try another server
				current = (current + 1) % t.numNodes
				// If we've looped around the entire way and we're not done,
				// then delay before bothering the other trackers again
				if current == t.nodeID {
					loops++
					current = (current + 1) % t.numNodes
					time.Sleep(time.Second * time.Duration(loops * REGISTER_PERIOD))
				}
			}
		}
	}
}

// Sends prepare messages to everyone
// Sends results back through the channel
func (t *trackerServer) preparePhase(done chan *PaxosDone) {

}

// Sends accept messages to everyone
// Sends results back through channel
func (t *trackerServer) acceptPhase(done chan *PaxosDone, op &trackerrpc.Operation) {

}

// Sends commit messages to everyone
// Writes back when it gets at least one reply
func (t *trackerServer) commitPhase(done chan *PaxosDone, op &trackerrpc.Operation) {

}

func (t *trackerServer) paxosCommunicator() {

}

func (t *trackerServer) paxosHandler() {
	pendingOps := list.New()
	initPaxos := make(chan struct{})
	prepareDone := make(chan *PaxosDone)
	acceptDone := make(chan *PaxosDone)
	commitDone := make(chan *PaxosDone)

	prepareReply := make(chan *PaxosReply)
	acceptReply := make(chan *PaxosReply)
	commitReply := make(chan *PaxosReply)
	inPaxos := false
	backoff := 1
	responses := 0
	accN := 0
	accV := trackerrpc.Operation{Type: trackerrpc.None}
	oks := 0
	for {
		select {
		case op := <-t.pending:
			pendingOps.PushBack(op)
			if !inPaxos {
				initPaxos <- struct{}{}
			}
		case <-initPaxos:
			inPaxos = true
			responses = 0
			// TODO
			// Needs to update t.myN
			for ch := range t.paxCom {
				ch <- &PaxosBroadcast{
					Type: Prepare,
					Reply: prepareReply}
			}
		case prep := <-prepareReply:
			if prep.ReqPaxNum == t.myN {
				responses++
				if prep.Status == trackerrpc.OK {
					oks++
					if prep.Value.Type != trackerrpc.None {
						if prep.PaxNum > accN {
							accN = prep.PaxNum
							accV = prep.Value
						}
					}
				} else if prep.Status == trackerrpc.OutOfDate {
					t.outOfDate <- prep.SeqNum
				}

				if responses == t.numNodes {
					if oks > (t.numNodes / 2) {
						if AccV.OpType == trackerrpc.None {
							e := pendingOps.Front()
							pendingOps.Remove(e)
							AccV = e.Value.(trackerrpc.Operation)
						}

						responses = 0
						oks = 0
						for ch := range t.paxCom {
							ch <- &PaxosBroadcast{
								Type: Accept,
								Reply: acceptReply,
								Value: &AccV}
						}
					} else {
						// Did not get quorum,
						// So wait (with exponential backoff) and try again
						backoff = 2*backoff
						wait := time.Second * time.Duration(backoff * REGISTER_PERIOD)
						time.AfterFunc(wait, func () { initPaxos <- struct{}{} })
					}
				}
			}
		case acc := <-acceptReply:
			if acc.ReqPaxNum == t.myN {
				responses++
				if acc.Status == trackerrpc.OK {
					oks++
				}

				if oks > (t.numNodes / 2) {
					for ch := range t.paxCom {
						ch <- &PaxosBroadcast{
							Type: Commit,
							Value: &AccV}
					}

					backoff = 1
					if pendingOps.Len() > 0 {
						// TODO
						// Make it skip prepare phase
						initPaxos <- struct{}{}
					} else {
						inPaxos = false
					}
				} else {
					backoff = 2*backoff
					wait := time.Second * time.Duration(backoff * REGISTER_PERIOD)
					time.AfterFunc(wait, func () { initPaxos <- struct{}{} })
				}
			}
		}
	}
}

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
	Reply chan *trackerrpc.UpdateReply
}

type Report struct {
	Args *trackerrpc.ReportArgs
	Reply chan *trackerrpc.UpdateReply
}

type Create struct {
	Args *trackerrpc.CreateArgs
	Reply chan *trackerrpc.UpdateReply

type Pending struct {
	Value trackerrpc.Operation
	Reply *UpdateReply
}

type PaxosReply struct {
	Status    trackerrpc.Status
	ReqPaxNum int
	PaxNum    int
	Value     trackerrpc.Operation
	SeqNum    int
}

type PaxosBroadcast struct {
	MyN    int
	Type   PaxosType
	Value  trackerrpc.Operation
	SeqNum int
	Reply  chan *PaxosReply
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
	creates              chan *Create
	pending              chan *Pending
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
	pendingOps           *list.List
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
		creates:              make(chan *Create),
		myN:                  nodeID,
                highestN:             0,
		accV:                 trackerrpc.Operation{OpType: trackerrpc.None},
		seqNum:               0,
		log:                  make(map[int]trackerrpc.Opeartion),
		numChunks:            make(map[string]int),
		peers:                make(map[string](map[string](struct{}))),
		trackers:             make([]*rpc.Client, numNodes),
		outOfDate:            make(chan int),
		paxCom:               make([](chan *PaxosBroadcast), numNodes)
		pendingOps:           list.New()
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

func (t *trackerServer) Prepare(args *trackerrpc.PrepareArgs, reply *trackerrpc.PrepareReply) error {
	replyChan := make(chan *trackerrpc.PrepareReply)
	prepare := &Prepare{
		Args:  args,
		Reply: replyChan}
	t.prepares <- prepare
	*reply = *(<-replyChan)
	return nil
}

func (t *trackerServer) Accept(args *trackerrpc.AcceptArgs, reply *trackerrpc.AcceptReply) error {
	replyChan := make(chan *trackerrpc.AcceptReply)
	accept := &Accept{
		Args:  args,
		Reply: replyChan}
	t.accepts <- accept
	*reply = *(<-replyChan)
	return nil
}

func (t *trackerServer) Commit(args *trackerrpc.CommitArgs, reply *trackerrpc.CommitReply) error {
	replyChan := make(chan *trackerrpc.CommitReply)
	commit := &Commit{
		Args:  args,
		Reply: replyChan}
	t.commits <- commit
	*reply = *(<-replyChan)
	return nil
}

func (t *trackerServer) ReportMissing(args *trackerrpc.ReportArgs, reply *trackerrpc.UpdateReply) error {
	replyChan := make(chan *trackerrpc.UpdateReply)
	report := &Report{
		Args:  args,
		Reply: replyChan}
	t.reports <- report
	*reply = *(<-replyChan)
	return nil
}

func (t *trackerServer) ConfirmChunk(args *trackerrpc.ConfirmArgs, reply *trackerrpc.UpdateReply) error {
	replyChan := make(chan *trackerrpc.UpdateReply)
	confirm := &Confirm{
		Args:  args,
		Reply: replyChan}
	t.confirms <- confirm
	*reply = *(<-replyChan)
	return nil
}

func (t *trackerServer) CreateEntry(args *trackerrpc.CreateArgs, reply *trackerrpc.UpdateReply) error {
	replyChan := make(chan *trackerrpc.UpdateReply)
	create := &Create{
		Args: args,
		Reply: replyChan}
	t.creates <- create
	*reply = *(<-replyChan)
	return nil
}

func (t *trackerServer) RequestChunk(args *trackerrpc.RequestArgs, reply *trackerrpc.RequestReply) error {
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
			// Handle prepare messages
			reply := &trackerrpc.PrepareReply{
				PaxNum: t.accN,
				Value:  t.accV,
				SeqNum: t.seqNum}
			if prep.Args.SeqNum != t.seqNum {
				if prep.Args.SeqNum < t.seqNum {
					// Other guy is out of date,
					// Let him know and send the correct value
					reply.Status = trackerrpc.OutOfDate
					reply.Value = t.log[prep.Args.SeqNum]
					prep.Reply <- reply
				} else {
					// This tracker is out of date
					// It should not be validating updates
					reply.Status = trackerrpc.Reject
					prep.Reply <- reply
					t.outOfDate <- prep.Args.SeqNum
				}
			} else if prep.Args.PaxNum < t.highestN {
				reply.Status = trackerrpc.Reject
				prep.Reply <- reply
			} else {
				t.highestN = prep.Args.PaxNum
				reply.Status = trackerrpc.OK
				prep.Reply <- reply
			}
		case acc := <-t.accepts:
			// Handle accept messages
			if acc.Args.SeqNum < t.seqNum {
				status = trackerrpc.OutOfDate
			} else if acc.Args.PaxNum < t.highestN {
				status = trackerrpc.Reject
			} else {
				status = trackerrpc.OK
				t.highestN = acc.Args.PaxNum
				t.accN = acc.Args.PaxNum
				t.accV = acc.Args.Value
			}
			acc.Reply <- &trackerrpc.AcceptReply{Status: status}
		case com := <-t.commits:
			// Handle commit messages
			v := com.Args.Value
			if v.SeqNum == t.seqNum {
				t.commitOp(v)
			}
			com.Reply <- &trackerrpc.CommitReply{}
		case get := <-t.gets:
			// Another tracker has requested a previously commited op
			s := gets.Args.SeqNum
			if s > t.seqNum {
				get.Reply <- &trackerrpc.GetReply{Status: trackerrpc.OutOfDate}
				t.outOfDate <- s
			} else {
				get.Reply <- &trackerrpc.GetReply{
					Status: trackerrpc.OK,
					Value:  t.log[s]}
			}
		case rep := <-t.reports:
			// A client has reported that it does not have a chunk
			numChunks, ok := t.numChunks[rep.Args.ID]
			if !ok {
				// File does not exist
				rep.Reply <- &trackerrpc.UpdateReply{Status: trackerrpc.FileNotfound}
			} else if req.Args.ChunkNum < 0 || req.Args.ChunkNum >= numChunks {
				// ChunkNum is not right for this file
				rep.Reply <- &trackerrpc.UpdateReply{Status: trackerrpc.OutOfRange}
			} else {
				// Put the operation in the pending list
				op := &trackerrpc.Operation{
					OpType: trackerrpc.Delete,
					ID: rep.Args.ID,
					ChunkNum: rep.Args.ChunkNum,
					ClientAddr: rep.Args.Addr}
				t.pending <- &Pending{
					Value: op,
					Reply: rep.Reply}
			}
		case conf := <-t.confirms:
			// A client has confirmed that it has a chunk
			numChunks, ok := t.numChunks[conf.Args.ID]
			if !ok {
				// File does not exist
				conf.Reply <- &trackerrpc.UpdateReply{Status: trackerrpc.FileNotfound}
			} else if conf.Args.ChunkNum < 0 || conf.Args.ChunkNum >= numChunks {
				// ChunkNum is not right for this file
				conf.Reply <- &trackerrpc.UpdateReply{Status: trackerrpc.OutOfRange}
			} else {
				// Put the operation in the pending list
				op := &trackerrpc.Operation{
					OpType: trackerrpc.Add,
					ID: conf.Args.ID,
					ChunkNum: conf.Args.ChunkNum,
					ClientAddr: conf.Args.Addr}
				t.pending <- &Pending{
					Value: op,
					Reply: conf.Reply}
			}
		case cre := <-t.creates:
			// A client has requested to create a new file
			numChunks, ok := t.numChunks[conf.Args.ID]
			if !ok {
				// ID not in use,
				// So make the pending request for this
				op := &trackerrpc.Operation{
					OpType: trackerrpc.Create,
					ID: cre.Args.ID
					ChunkNum: cre.Args.NumChunks}
				t.pending <- &Pending{
					Value: op,
					Reply: cre.Reply}
			} else {
				// File already exists, so tell the client that this ID is invalid
				cre.Reply <- &trackerrpc.UpdateReply{Status: trackerrpc.InvalidID}
			}
		case req := <-t.requests:
			// A client has requested a list of users with a certain chunk
			numChunks, ok := t.numChunks[req.Args.ID]
			if !ok {
				// File does not exist
				req.Reply <- &trackerrpc.RequestReply{Status: trackerrpc.FileNotfound}
			} else if req.Args.ChunkNum < 0 || req.Args.ChunkNum >= numChunks {
				// ChunkNum is not right for this file
				req.Reply <- &trackerrpc.RequestReply{Status: trackerrpc.OutOfRange}
			} else {
				// Get a list of all peers, then respond
				key := req.Args.ID + ":" + strconv.Itoa(req.Args.ChunkNum)
				peers := make([]string, 0)
				for k, _ := range t.peers[key] {
					append(peers, k)
				}
				req.Reply <- &trackerrpc.RequestReply{
					Status: trackerrpc.OK,
					Peers: peers}
			}
		case seqNum := <-t.outOfDate:
			// t is out of date
			// Needs to catch up to seqNum
			t.catchUp(seqNum)
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
	m, ok := t.peers[key]
	if !ok {
		t.peers[key] = make(map[string](struct{}))
		m = t.peers[key]
	}

	if v.OpType == trackerrpc.Add {
		m[v.ClientAddr] = struct{}{}
	} else if v.OpType == trackerrpc.Remove {
		delete(m,v.ClientAddr)
	} else if v.OpType == trackerrpc.Create {
		t.numChunks[v.ID] = v.ChunkNum
	}

	// Go through the list of ops that we have pending
	// If this is one of those, then respond
	done := false
	for e := t.pendingOps.Front(); !done && e != nil; e = e.Next() {
		pen := e.Value.(*Pending).Value
		if pen.OpType == v.OpType && pen.ID == v.ID && pen.ChunkNum == v.ChunkNum && pen.ClientAddr == v.ClientAddr {
			done = true
			t.pendingOps.Remove(e)
			e.Value.(*Pending).Reply <- &trackerrpc.UpdateReply{Status: trackerrpc.OK}
		}
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

// A function to send Prepare, Accept, and Commit messages
// to a single node in the Paxos cluster
func (t *trackerServer) paxosCommunicator(id int) {
	for {
		select {
		case mess := <-t.paxCom[id]:
			reqPaxNum := mess.MyN
			if mess.Type == Prepare {
				args := &trackerrpc.PrepareArgs{
					PaxNum: reqPaxNum,
					SeqNum: mess.seqNum}
				reply := &trackerrpc.PrepareReply{}
				if err := t.trackers[id].Call("Paxos.Prepare", args, reply); err != nil {
					// Error: Tell the paxosHandler that we were "rejected"
					mess.Reply <- &PaxosReply{Status: trackerrpc.Reject}
				} else {
					// Pass the data back to the PaxosHandler
					mess.Reply <- &PaxosReply{
						Status: reply.Status,
						ReqPaxNum: reqPaxNum,
						PaxNum: reply.PaxNum,
						Value: reply.Value,
						SeqNum: reply.SeqNum}
				}
			} else if mess.Type == Accept {
				args := &trackerrpc.AcceptArgs{
					PaxNum: reqPaxNum,
					SeqNum: mess.seqNum,
					Value: mess.Value}
				reply := &trackerrpc.AcceptReply{}
				if err := t.trackers[id].Call("Paxos.Accept", args, reply); err != nil {
					// Error: Tell the paxosHandler that we were "rejected"
					mess.Reply <- &PaxosReply{Status: trackerrpc.Reject}
				} else {
					mess.Reply <- &PaxosReply{
						Status: reply.Status,
						ReqPaxNum: reqPaxNum,
						SeqNum: mess.seqNum}
				}
			} else if mess.Type == Commit {
				args := &trackerrpc.CommitArgs{
					SeqNum: mess.seqNum,
					Value: mess.Value}
				reply := &trackerrpc.CommitReply{}
				t.trackers[id].Call("Paxos.Commit", args, reply)
			}
		}
	}
}

func (t *trackerServer) paxosHandler() {
	pendingOps := list.New()
	initPaxos := make(chan struct{})
	prepareReply := make(chan *PaxosReply)
	acceptReply := make(chan *PaxosReply)
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
			t.pendingOps.PushBack(op)
			if !inPaxos {
				initPaxos <- struct{}{}
			}
		case <-initPaxos:
			inPaxos = true
			t.myN = (t.highestN - (t.highestN % t.numNodes)) + (t.numNodes + t.nodeID)
			backoff = 1
			responses = 0
			oks = 0
			for ch := range t.paxCom {
				ch <- &PaxosBroadcast{
					MyN: t.myN,
					Type: Prepare,
					Reply: prepareReply,
					SeqNum: t.seqNum}
			}
		case prep := <-prepareReply:
			// First check that this is a response to the current PaxosMessage
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

				if oks > (t.numNodes / 2) {
					if accV.OpType == trackerrpc.None {
						e := pendingOps.Front()
						pendingOps.Remove(e)
						accV = e.Value.(*Pending).Value
					}

					// Broadcast the accept message
					// First increment the myN,
					// So that replies to previous messages will be ignored
					t.myN += t.numNodes
					responses = 0
					oks = 0
					for ch := range t.paxCom {
						ch <- &PaxosBroadcast{
							MyN: t.myN,
							Type: Accept,
							Reply: acceptReply,
							SeqNum: t.seqNum,
							Value: &accV}
					}
				} else {
					// Did not get quorum,
					// So wait (with exponential backoff) and try again
					backoff = 2*backoff
					wait := time.Second * time.Duration(backoff * REGISTER_PERIOD)
					time.AfterFunc(wait, func () { initPaxos <- struct{}{} })
				}
			}
		case acc := <-acceptReply:
			// Received the reply to an accept message
			if acc.ReqPaxNum == t.myN && inPaxos {
				responses++
				if acc.Status == trackerrpc.OK {
					oks++
				}

				if oks > (t.numNodes / 2) {
					for ch := range t.paxCom {
						ch <- &PaxosBroadcast{
							MyN: t.myN,
							Type: Commit,
							SeqNum: t.seqNum,
							Value: &accV}
					}

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

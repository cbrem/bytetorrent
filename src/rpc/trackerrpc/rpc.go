package trackerrpc

// These are the functions that Trackers will call on each other
type Paxos interface {
	RegisterServer(*RegisterArgs, *RegisterReply) error
	Prepare(*PrepareArgs, *PrepareReply) error
	Accept(*AcceptArgs, *AcceptReply) error
	Commit(*CommitArgs, *CommitReply) error
}

// These are the functions that Clients will call on Trackers
type Tracker interface {
	ReportMissing(*ReportArgs, *ReportReply) error
	ConfirmChunk(*ConfirmArgs, *ConfirmReply) error
	RequestChunk(*RequestArgs, *RequestReply) error
	// ValidateID(*ValidateArgs, *ValidateReply) error
}

type PaxosServer struct {
	Paxos
}

func WrapPaxos(p Paxos) Paxos {
	return &PaxosServer{p}
}

type TrackerServer {
	Tracker
}

func WrapTracker(t Tracker) Tracker {
	return &TrackerServer{t}
}

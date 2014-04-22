package trackerrpc

// These are the functions that Trackers will call on each other
type Paxos interface {
	RegisterServer(*RegisterArgs, *RegisterReply) error
	GetOp(*GetArgs, *GetReply) error
	Prepare(*PrepareArgs, *PrepareReply) error
	Accept(*AcceptArgs, *AcceptReply) error
	Commit(*CommitArgs, *CommitReply) error
}

// These are the functions that Clients will call on Trackers
type Tracker interface {
	ReportMissing(*ReportArgs, *UpdateReply) error
	ConfirmChunk(*ConfirmArgs, *UpdateReply) error
	RequestChunk(*RequestArgs, *RequestReply) error
	CreateEntry(*CreateArgs, *UpdateReply) error
	GetTrackers(*TrackersArgs, *TrackersReply) error
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

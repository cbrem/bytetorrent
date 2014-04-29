package tracker

import (
	"tracker/trackerproto"
)

// These are the functions that Trackers will call on each other
type PaxosTracker interface {
	RegisterServer(*trackerproto.RegisterArgs, *trackerproto.RegisterReply) error
	GetOp(*trackerproto.GetArgs, *trackerproto.GetReply) error
	Prepare(*trackerproto.PrepareArgs, *trackerproto.PrepareReply) error
	Accept(*trackerproto.AcceptArgs, *trackerproto.AcceptReply) error
	Commit(*trackerproto.CommitArgs, *trackerproto.CommitReply) error
}

// These are the functions that Clients will call on Trackers
type RemoteTracker interface {
	ReportMissing(*trackerproto.ReportArgs, *trackerproto.UpdateReply) error
	ConfirmChunk(*trackerproto.ConfirmArgs, *trackerproto.UpdateReply) error
	RequestChunk(*trackerproto.RequestArgs, *trackerproto.RequestReply) error
	CreateEntry(*trackerproto.CreateArgs, *trackerproto.UpdateReply) error
	GetTrackers(*trackerproto.TrackersArgs, *trackerproto.TrackersReply) error
}

type WrappedPaxosTracker struct {
	PaxosTracker
}

type WrappedRemoteTracker struct {
	RemoteTracker
}

func WrapPaxos(pt PaxosTracker) PaxosTracker {
	return &WrappedPaxosTracker{pt}
}

func WrapRemote(t RemoteTracker) RemoteTracker {
	return &WrappedRemoteTracker{t}
}

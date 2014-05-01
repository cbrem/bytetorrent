package dummytracker

import (
    "tracker/trackerproto"
)

// Dummy Trackers will handle RPCs on this interface.
type DummyTracker interface {
    ReportMissing(*trackerproto.ReportArgs, *trackerproto.UpdateReply) error
    ConfirmChunk(*trackerproto.ConfirmArgs, *trackerproto.UpdateReply) error
    RequestChunk(*trackerproto.RequestArgs, *trackerproto.RequestReply) error
    CreateEntry(*trackerproto.CreateArgs, *trackerproto.UpdateReply) error
    GetTrackers(*trackerproto.TrackersArgs, *trackerproto.TrackersReply) error
}

type WrappedDummyTracker struct {
    DummyTracker
}

// Returns a wrapped RemoteClient which is guaranteed to handle RPCs on only
// the puglic RemoteClient requests.
func Wrap(c DummyTracker) DummyTracker {
    return & WrappedDummyTracker{c}
}

package tracker

import "tracker/trackerproto"

type Tracker interface {
	// RegisterServer adds a Tracker to the Paxos cluster.
	// Repiles with a list of all host:ports in the cluster.
	// Returns status:
	// - OK: If everything worked
	// - NotReady: If the cluster is still setting up
	RegisterServer(*trackerproto.RegisterArgs, *trackerproto.RegisterReply) error

	// GetOp returns the operation processed at the requested SeqNum
	// Returns status:
	// - OK: If everything worked
	// - OutOfDate: If the server does not have that SeqNum in the log
	GetOp(*trackerproto.GetArgs, *trackerproto.GetReply) error

	// Prepare returns:
	// - <Reject, _, _> : If PaxNum < Highest PaxNum seen
	// - <OutOfDate, _, V> : If SeqNum < current SeqNum
        //                       V is the value committed at that point in the sequence
	// - <OK, N, V> : If PaxNum >= Highest PaxNum seen
	//                (N,V) is (PaxNum, Value) pair of highest accepted proposal
	Prepare(*trackerproto.PrepareArgs, *trackerproto.PrepareReply) error

	// Accept returns:
	// - <Reject> : If PaxNum < Highest PaxNum seen
	// - <OutOfDate> : If SeqNum < current SeqNum
	// - <OK> : Otherwise (everything went well)
	Accept(*trackerproto.AcceptArgs, *trackerproto.AcceptReply) error

	// Commits a change to local memory.
	// Does not return anything
	Commit(*trackerproto.CommitArgs, *trackerproto.CommitReply) error

	// ReportMissing allows the Client to inform the Tracker when it
	// does not possess a chunk that other Clients think it has.
	// This function will block until the Paxos ring has acknoledged the change.
	// Returns status:
	// - OK: If everything is good
	// - FileNotFound: ID is not a valid file
	// - OutOfRange: The chunk number was to high (or negative)
	ReportMissing(*trackerproto.ReportArgs, *trackerproto.UpdateReply) error

	// ConfirmChunk allows the Client to inform the Tracker when it
	// comes into possession of the a chunk.
	// This function will block until the Paxos ring has acknoweledged the change
	// Returns status:
	// - OK: If everything is good
	// - FileNotFound: ID is not a valid file
	// - OutOfRange: The chunk number was to high (or negative)
	ConfirmChunk(*trackerproto.ConfirmArgs, *trackerproto.UpdateReply) error

	// RequestChunk returns a slice of peers with the requested chunk for the file
	// Returns status:
	// - OK: If everything is good
	// - FileNotFound: ID is not a valid file
	// - OutOfRange: The chunk number was too high (or negative)
	RequestChunk(*trackerproto.RequestArgs, *trackerproto.RequestReply) error

	// CreateEntry creates an entry on the tracker for a new torrent.
	// Blocks until the option has been committed
	// Returns status:
	// - OK: If an entry was successfully created for the torrent with the
	//   given ID
	// - InvalidID: If there is already a torrent with this ID
	// - InvalidTrackers: If the supplied list of trackers does not match the cluster
	CreateEntry(*trackerproto.CreateArgs, *trackerproto.UpdateReply) error

	// GetTrackers returns a list of all trackers in the cluster
	// Returns status OK, unless something went horribly wrong
	GetTrackers(*trackerproto.TrackersArgs, *trackerproto.TrackersReply) error

	// Lets you stall a tracker
	// If 0 is passed, the tracker is shut down
	// Should only be used for testing
	DebugStall(int)
}

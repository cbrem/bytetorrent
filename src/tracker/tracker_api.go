package tracker

import "rpc/trackerrpc"

type Tracker interface {
	// RegisterServer adds a Tracker to the Paxos cluster.
	// Repiles with a list of all host:ports in the cluster.
	// Returns status:
	// - OK: If everything worked
	// - NotReady: If the cluster is still setting up
	RegisterServer(*RegisterArgs, *RegisterReply) error

	// GetOp returns the operation processed at the requested SeqNum
	// Returns status:
	// - OK: If everything worked
	// - OutOfDate: If the server does not have that SeqNum in the log
	GetOp(*GetArgs, *GetReply) error

	// Prepare returns:
	// - <Reject, _, _> : If PaxNum < Highest PaxNum seen
	// - <OutOfDate, _, V> : If SeqNum < current SeqNum
        //                       V is the value committed at that point in the sequence
	// - <OK, N, V> : If PaxNum >= Highest PaxNum seen
	//                (N,V) is (PaxNum, Value) pair of highest accepted proposal
	Prepare(*PrepareArgs, *PrepareReply) error

	// Accept returns:
	// - <Reject> : If PaxNum < Highest PaxNum seen
	// - <OutOfDate> : If SeqNum < current SeqNum
	// - <OK> : Otherwise (everything went well)
	Accept(*AcceptArgs, *AcceptReply) error

	// Commits a change to local memory.
	// Does not return anything
	Commit(*CommitArgs, *CommitReply) error

	// ReportMissing allows the Client to inform the Tracker when it
	// does not possess a chunk that other Clients think it has.
	// This function will block until the Paxos ring has acknoledged the change.
	// Returns status:
	// - OK: If everything is good
	// - FileNotFound: ID is not a valid file
	// - OutOfRange: The chunk number was to high (or negative)
	ReportMissing(*ReportArgs, *ReportReply) error

	// ConfirmChunk allows the Client to inform the Tracker when it
	// comes into possession of the a chunk.
	// This function will block until the Paxos ring has acknoweledged the change
	// Returns status:
	// - OK: If everything is good
	// - FileNotFound: ID is not a valid file
	// - OutOfRange: The chunk number was to high (or negative)
	ConfirmChunk(*ConfirmArgs, *ConfirmReply) error

	// RequestChunk returns a slice of peers with the requested chunk for the file
	// Returns status:
	// - OK: If everything is good
	// - FileNotFound: ID is not a valid file
	// - OutOfRange: The chunk number was too high (or negative)
	RequestChunk(*RequestArgs, *RequestReply) error
}

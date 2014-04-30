package trackerproto

import "torrent/torrentproto"

type Status int

const (
	OK        Status = iota + 1 // RPC was a success
	Reject                      // Reject a prepare/accept request
	OutOfDate                   // Message was for committed slot
	NotReady                    // Trackers are still getting ready
	FileNotFound                // FileID does not exist
	OutOfRange                  // Chunk Number out of range for file
	InvalidID                   // ID is not valid
	InvalidTrackers             // List of trackers was invalid (for torrent creation)
)

type OperationType int

const (
	None   OperationType = iota
	Add
	Delete
	Create
)

type Operation struct {
	OpType     OperationType   // Type of operation
	Chunk      torrentproto.ChunkID // Torrent ID and chunk number
	ClientAddr string          // The host:port of the client in question
	Torrent    torrentproto.Torrent // The torrent information (if you're trying to create a torrent)
}

type Node struct {
	HostPort string // The host:port address of tracker node
	NodeID   int
}

type RegisterArgs struct {
	TrackerInfo Node
}

type RegisterReply struct {
	Status
	Trackers []Node
}

type GetArgs struct {
	SeqNum int
}

type GetReply struct {
	Status
	Value  Operation
}

type PrepareArgs struct {
	PaxNum int
	SeqNum int
}

type PrepareReply struct {
	Status
	PaxNum int
	Value  Operation
	SeqNum int
}

type AcceptArgs struct {
	PaxNum int
	SeqNum int
	Value  Operation
}

type AcceptReply struct {
	Status
}

type CommitArgs struct {
	SeqNum int
	Value  Operation
}

type CommitReply struct {
	// Intentionally Blank
}

type ReportArgs struct {
	Chunk    torrentproto.ChunkID // Torrent ID and chunk number
	HostPort string          // host:port of the client
}

type ConfirmArgs struct {
	Chunk    torrentproto.ChunkID // Torrent ID and chunk number
	HostPort string          // host:port of the client
}

type RequestArgs struct {
	Chunk torrentproto.ChunkID // Torrent ID and chunk number
}

type RequestReply struct {
	Status
	Peers  []string // A list of host:port of peers with chunk
	ChunkHash string // The definitive hash for this chunk
}

type CreateArgs struct {
	Torrent torrentproto.Torrent
}

type UpdateReply struct {
	Status
}

type TrackersArgs struct {
	// Intentionally Blank
}

type TrackersReply struct {
	Status    Status
	HostPorts []string
}

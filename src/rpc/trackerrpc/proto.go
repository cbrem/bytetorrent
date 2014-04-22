package trackerrpc

import "torrent"

type Status int

const (
	OK        Status = iota + 1 // RPC was a success
	Reject                      // Reject a prepare/accept request
	OutOfDate                   // Message was for committed slot
	NotReady                    // Trackers are still getting ready
	FileNotfound                // FileID does not exist
	OutOfRange                  // Chunk Number out of range for file
	InvalidID                   // ID is not valid
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
	Chunk      torrent.ChunkID // Torrent ID and chunk number
	ClientAddr string          // The host:port of the client in question
}

type Node struct {
	HostPort string // The host:port address of tracker node
	NodeID   int
}

type RegisterArgs struct {
	TrackerInfo Node
}

type RegisterReply struct {
	Status   Status
	Trackers []Node
}

type GetArgs struct {
	SeqNum int
}

type GetReply struct {
	Status status
	Value  Operation
}

type PrepareArgs struct {
	PaxNum int
	SeqNum int
}

type PrepareReply struct {
	Status Status
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
	Status Status
}

type CommitArgs struct {
	SeqNum int
	Value  Operation
}

type CommitReply struct {
	// Intentionally Blank
}

type ReportArgs struct {
	Chunk    torrent.ChunkID // Torrent ID and chunk number
	HostPort string          // host:port of the client
}

type ConfirmArgs struct {
	Chunk    torrent.ChunkID // Torrent ID and chunk number
	HostPort string          // host:port of the client
}

type RequestArgs struct {
	Chunk torrent.ChunkID // Torrent ID and chunk number
}

type RequestReply struct {
	Status Status
	Peers  []string // A list of host:port of peers with chunk
}

type CreateArgs struct {
	Torrent torrent.Torrent
}

type UpdateReply struct {
	Status Status
}

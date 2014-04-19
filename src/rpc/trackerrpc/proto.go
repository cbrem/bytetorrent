package trackerrpc

type Status int

const (
	OK        Status = iota + 1 // RPC was a success
	Reject                      // Reject a prepare/accept request
	OutOfDate                   // Message was for committed slot
	NotReady                    // Trackers are still getting ready
	FileNotFound                // ID does not exist
	OutOfRange                  // Chunk Number out of range for file
)

type Operation struct {
	// TODO
}

type Node struct {
	HostPort string // The host:port address of tracker node
}

type RegisterArgs struct {
	TrackerInfo Node
}

type RegisterReply struct {
	Status   Status
	Trackers []Node
}

type PrepareArgs struct {
	PaxNum int
	SeqNum int
}

type PrepareReply struct {
	Status Status
	PaxNum int
	Value  Operation
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
	ID       string // Unique ID for the torrent
	ChunkNum int
}

type ReportReply struct {
	Status Status
}

type ConfirmArgs struct {
	ID       string // Unique ID for the torrent
	ChunkNum int
}

type ConfirmReply struct {
	Status Status
}

type RequestArgs struct {
	ID       string // Unique ID for the torrent
	ChunkNum int
}

type RequestReply struct {
	Status Status
	Peers  []string // A list of host:port of peers with chunk
}

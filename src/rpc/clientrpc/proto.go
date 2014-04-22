package clientrpc

type Status int

const (
    OK        Status = iota + 1 // RPC was a success
    ChunkNotFound               // The requested chunk is not available
)

type GetArgs struct {
    ID         string        // Unique ID for the relevant torrent file
    ChunkNum   int           // The ChunkNum being updated
}

type GetReply struct {
    Status Status
    Chunk []byte
}

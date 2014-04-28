package clientrpc

import (
    "torrent"
)

type Status int
const (
    OK        Status = iota + 1 // RPC was a success
    ChunkNotFound               // The requested chunk is not available
)

type GetArgs struct {
    torrent.ChunkID // ID and chunk number for the relevant torrent chunk
}

type GetReply struct {
    Status Status
    Chunk []byte
}

package clientproto

import (
    "torrent/torrentproto"
)

// Types of operations which can be performed on local files.
type Operation int
const (
    LocalFileAdd Operation = iota + 1
    LocalFileDelete
    LocalFileUpdate
)

// Statuses for client RPCs.
type Status int
const (
    OK        Status = iota + 1 // RPC was a success
    ChunkNotFound               // The requested chunk is not available
)

// Local representation of a torrented/torrentable file.
// Contains information about a Torrent and portions of the corresponding file
// present locally.
type LocalFile struct {
    Torrent torrentproto.Torrent
    Path string // Path to a local copy of the file
    Chunks map[int]struct{} // Indicates whether this client possesses each chunk.
}

// Information about a change to a local file.
// All changes describe some operation which was performed on a file.
type LocalFileChange struct {
    Operation
    *LocalFile
}

// Information about a GetChunks RPC
type GetArgs struct {
    torrentproto.ChunkID // ID and chunk number for the relevant torrent chunk
}

// Information about a GetChunks RPC result
type GetReply struct {
    Status Status
    Chunk []byte
}

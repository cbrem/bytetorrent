package client

import (
    "src/torrent"
)

// Types of operations which can be performed on local files.
type LocalFileOperation int
const (
    LocalFileAdd LocalFileOperation = iota + 1
    LocalFileDelete
    LocalFileUpdate
)

// Local representation of a torrented/torrentable file.
// Contains information about a Torrent and portions of the corresponding file
// present locally.
type LocalFile struct {
    Torrent *torrent.Torrent
    Path string // Path to a local copy of the file
    Chunks map[int]struct{} // Indicates whether this client possesses each chunk.
}

// Information about a change to a local file.
// All changes describe some operation which was performed on a file.
type LocalFileChange struct {
    LocalFileOperation
    *LocalFile
}

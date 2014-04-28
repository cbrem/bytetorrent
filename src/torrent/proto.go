package torrent

// Information about one node in a tracker.
type TrackerNode struct {
    HostPort string
}

// A Key which uniquely identifies a Torrent.
// It has the form <name, file_hash>.
type ID struct {
    Name string // A human-readable name for this Torrent
    Hash []byte // The SHA-1 hash of the file associated with the Torrent
}

// An identifier for a chunk within a torrent.
type ChunkID struct {
    ID
    ChunkNum int
}

// A deserialized .torrent file.
// Contains information about how to fetch 
type Torrent struct {
    ID
    ChunkHashes map[int]byte
    TrackerNodes []TrackerNode // The nodes in the tracker with which this torrent is registered
    ChunkSize int
    FileSize int
}

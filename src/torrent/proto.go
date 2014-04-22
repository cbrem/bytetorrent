package torrent

const (
    DEFAULT_CHUNK_SIZE int 1000 // Number of bytes per chunk
)

// Information about one node in a tracker.
type TrackerNode {
    HostPort string
}

// A deserialized .torrent file.
// Contains information about how to fetch 
type Torrent struct {
    Name string // A human-readable name
    ID string // A identifier that is unique for the tracker 
    Hash []byte // The SHA-1 hash of the associated file
    ChunkHashes [][]byte
    TrackerNodes []TrackerNode // The nodes in the tracker with which this torrent is registered
    ChunkSize int
    FileSize int
}

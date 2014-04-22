package torrent

// Information about one node in a tracker.
type TrackerNode {
    HostPort string
}

// A deserialized .torrent file.
// Contains information about how to fetch 
type Torrent struct {
    name string // A human-readable name
    ID string // A identifier that is unique for the tracker 
    hash uint32 // The SHA-1 hash of the associated file
    trackerNodes []TrackerNode // The nodes in the tracker with which this torrent is registered
}

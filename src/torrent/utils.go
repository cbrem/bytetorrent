package torrent

// New creates a new Torrent for the file at the given path.
// Gives this Torrent the given human-readable name.
// Throws an error if this name is not valid.
// Note that we do not check here to see if trackerNodes are valid or if the
// given name results in an ID that is valid for trackerNodes.
func New(path string, name string, trackerNodes []TrackerNode) (Torrent, error) {
    // TODO
}

// TODO: to/from string methods?

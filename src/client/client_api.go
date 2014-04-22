package client

// TODO: is the current torrent/client/tracker relationship good?
// would it make more sense to create the torrent and register it on the tracker
// in one fell swoop?

// The ByteTorrent Client API.
type Client interface {
    // GetChunk gets a chunk (i.e. a array of consecutive bytes) of a data file.
    // Returns Status:
    // - OK: If the reply contains the requested chunk.
    // - ChunkNotFound: If the Client does not contain the requested chunk for
    //   the requested file.
    GetChunk(*clientrpc.GetChunkArgs, *clientrpc.GetChunkReply) error

    // RegisterFile registers with the Client that the file with the given
    // Torrent is available at the given path.
    // It also attempts to register the Torrent with the tracker nodes listed
    // in the torrent.
    // After this function is called, other clients will be able to get chunks
    // of this file from this client.
    // Throws an error if:
    // - a file which matches the Torrent is not found at this path
    // - the tracker nodes listed in the torrent do not accept the torrent
    RegisterTorrent(*torrent.Torrent, path string) error

    // DownloadFile downloads the file with the given Torrent, and stores it at
    // the given path.
    // Throws an error if the torrent is not valid.
    // Otherwise, download the file asynchronously and pushes the the given
    // channel when done.
    DownloadFile(*torrent.Torrent, path string, done <-chan struct{}) error
}

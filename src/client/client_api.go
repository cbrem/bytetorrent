package client

// The ByteTorrent Client API.
type Client interface {
    // GetChunk gets a chunk (i.e. a array of consecutive bytes) of a data file.
    // Returns Status:
    // - OK: If the reply contains the requested chunk.
    // - ChunkNotFound: If the Client does not contain the requested chunk for
    //   the requested file.
    GetChunk(*clientrpc.GetChunkArgs, *clientrpc.GetChunkReply) error

    // OfferFile associates a local file with a Torrent within the Client.
    // It also informs the trackerNodes listed in the Torrent that this Client
    // posesses the file.
    // After this function is called, other clients will be able to get chunks
    // of this file from this Client.
    // Throws an error if:
    // - a file is not found at the given path
    // - the Torrent is not valid (e.g. if the trackerNodes given in the Torrent
    //   do not exist, or have no record of this Torrent)
    OfferFile(*torrent.Torrent, path string) error

    // DownloadFile downloads the file with the given Torrent, and stores it at
    // the given path.
    // Blocks until the file has completely downloaded.
    // Throws an error if:
    // - the given torrent is not valid
    // - the given path is not valid
    DownloadFile(*torrent.Torrent, path string) error
}

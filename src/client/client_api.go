package client

import (
    "client/clientproto"
    "torrent/torrentproto"
)

// The ByteTorrent Client API.
// Clients operate based on trust of Trackers and distrust of/disregard for
// other Clients. Clients believe all data from Trackers, but check all data
// from other clients against associated hashes to make sure that it has not
// been tampered with. This distrust is well-founded, for Clients make no
// attempt to ensure that the data they send to other Clients is valid or safe.
type Client interface {
    // GetChunk gets a chunk (i.e. a array of consecutive bytes) of a data file.
    // Returns Status:
    // - OK: If the reply contains the requested chunk.
    // - ChunkNotFound: If the Client does not contain the requested chunk for
    //   the requested file.
    GetChunk(*clientproto.GetArgs, *clientproto.GetReply) error

    // OfferFile associates a local file with a Torrent within the Client.
    // It also informs the trackerNodes listed in the Torrent that this Client
    // posesses the file.
    // After this function is called, other clients will be able to get chunks
    // of this file from this Client.
    // Does not check if the torrent or file are valid.
    // Throws an error if the Client cannot inform trackerNodes that it
    // possesses this file (e.g. it cannot reach trackerNodes, or trackerNodes
    // do not know about this torrent).
    OfferFile(torrentproto.Torrent, string) error

    // DownloadFile downloads the file with the given Torrent, and stores it at
    // the given path.
    // Blocks until the file has completely downloaded.
    // Throws an error if:
    // - the given torrent is not valid
    // - the given path is not valid
    DownloadFile(torrentproto.Torrent, string) error

    // Close shuts down this Client in an orderly manner.
    // It writes the Client's state out to a file.
    // Close throws an error if it is not able to write the Client's state to a
    // file.
    Close() error
}

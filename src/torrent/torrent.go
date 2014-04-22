// TODO: Currently just hashing the whole file. Should we hash
// each chunk instead? i think we need to in the end...because this
// enables clients to start seeding chunks before they have the
// whole file

package torrent

import (
    "crypto/sha1"
    "io/ioutil"
    "rpc"
)

// New creates a new Torrent for the file at the given path.
// Gives this Torrent the given human-readable name.
// If this function returns without error, the Torrent returned is guaranteed
// to uniquely identify this file on the given trackerNodes.
// Throws an error if:
// - no file exists at this path
// - trackerNodes do not exist or cannot be contacted
// - the given file and name are not valid for trackerNodes
//
// TODO: major design decision here - make sure it's a good idea:
// when we create a Torrent, we check in with the trackerNodes that it's going
// to reference to make sure that there isn't already a Torrent with this
// (name, file_hash) (i.e. the same id). if we didn't do this, then it would be
// possible (though very difficult) to create two torrents for different files
// which share the same entry on the trackers.
func New(path string, name string, trackerNodes []TrackerNode) (Torrent, error) {
    t := & Torrent {
        Name: name,
        TrackerNodes: trackerNodes[:],
        ChunkSize: DEFAULT_CHUNK_SIZE} // TODO: do we need to copy here?

    // Attempt to find the file with the given path.
    var file []byte
    if file, err := ioutil.ReadFile(path); err != nil {
        // Failed to read the file at the given path.
        return err
    }
    t.FileSize := len(file)

    // Hash the entire file we just read, as well as each chunk.
    // Save all results in the Torrent.
    h := sha1.New()
    h.Write(file)
    t.Hash := h.Sum(nil)
    t.ChunkHashes = make([][]byte, 0)
    for i := 0; i < t.NumChunks(); i++ {
        chunk := t.GetChunk(path, i)
        h.Reset()
        h.Write(chunk)
        t.ChunkHashes = append(t.ChunkHashes, h.Sum(nil))
    }

    // Set the Torrent's ID equal to the hash concatenated with the name.
    t.ID = strings.Join([]string{string(t.Hash), name}, "")

    // Attempt to contact one of the tracker nodes and create an entry for this
    // ID.
    //
    // TODO: only being able to create a torrent with a given ID once is fairly fragile
    // ...if we somehow forget that we created the torrent, our guarantees break... and
    // what if we do create on the tracker, but the client doesn't find out...would it
    // be blocked from ever creating a torrent with the name it wants to?
    for _, trackerNode := range trackerNodes {
        if conn, err := rpc.DialHTTP("tcp", trackerNode.HostPort); err == nil {
            // We found a live node in the tracker cluster.
            args := & trackerrpc.CreateArgs {ID: t.ID, NumChunks: t.NumChunks()}
            reply :=  & trackerrpc.UpdateReply {}
            if err := conn.Call("Tracker.CreateEntry", args, reply); err == nil {
                // Tracker responded to CreateEntry call. If create was successful,
                // continue creating Torrent. Otherwise, return an error.
                switch reply.Status {
                case trackerrpc.OK: return (t, nil)
                case trackerrpc.InvalidID: return (nil, errors.New("Cannot create Torrent: invalid ID"))
                }
            }
        }
    }

    // No trackers responded. Cannot create Torrent.
    return (nil, errors.New("Cannot create Torrent: Tracker unresponsive"))
}

// NumChunks returns the number of chunks into which we this Torrent's file is
// divided.
func (t *Torrent) NumChunks() {
    if t.FileSize % t.ChunkSize == 0 {
        return t.FileSize / t.ChunkSize, 
    } else {
        // Round up.
        return (t.FileSize / t.ChunkSize) + 1
    }
}

// GetChunk returns a the chunk with the given number from this Torrent.
// If the given number is out of range, it returns a non-nil error.
func (t *Torrent) GetChunk(path string, chunkNumber int) ([]byte, error) {
    // Find the Chunk's start and end.
    var start, end int
    start = chunkNumber * t.ChunkSize
    if start + t.ChunkSize < t.FileSize {
        end = start + t.ChunkSize
    } else {
        end = t.FileSize
    }

    // Determine whether we're out of bounds.
    if start < 0 || end <= start {
        return (nil, errors.New("Cannot get chunk: bad chunk number"))
    }

    // Attempt to read in the file and return a slice.
    if file, err := ioutil.ReadFile(path); err != nil {
        // Failed to read the file at the given path.
        return err
    } else {
        return file[start : end]
    }
}

// TODO: to/from string methods?

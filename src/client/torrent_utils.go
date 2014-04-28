// TODO: Currently just hashing the whole file. Should we hash
// each chunk instead? i think we need to in the end...because this
// enables clients to start seeding chunks before they have the
// whole file

// TODO: store whole torrent on tracker upon creation.
// what's this good for? it addresses the following situation?
// - alice creates a torrent for game_of_thrones.mp4, and registers it
// - eve creates a copy of this torrent with different per-chunk hashes
//   that match the chunks in virus.exe, and offers virus.exe along with
//   this new torrent
// - bob gets some of his chunks from eve, and only realizes that they're
//   corrupted when he gets the whole file and notices that it doesn't match
//   the whole file hash
// Solution! Let clients chech when they get a torrent that it actually is
// a torrent that was created on the tracker.

// TODO: CreateEntry now offers a whole (unconfirmed, unregistered) torrent
// to try and get it approved by the tracker.
// if the tracker approves, it creates an entry, and New() can return the Torrent

// TODO: move a lot of this out into an activateTorrent function

// TODO: for efficiency, change getChunk to take a File object instead of
// opening/closing each time

// TODO: note that, if we have a torrent for a chunkID that another client is
// requesting, we know *exactly* which torrent that client used to request
// the chunk...because ChunkIDs contain torrent IDs, and torrentIDs are
// uniquely tied to torrents

// TODO: change size in torrent to an int64?

package client

import (
    "crypto/sha1"
    "errors"
    "os"
    "net/rpc"

    "rpc/trackerrpc"
)

const (
    DEFAULT_CHUNK_SIZE int = 1000 // Number of bytes per chunk
    MODE int = 644 // Mode for writing torrent files.
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
func New(path string, name string, trackerNodes []TrackerNode) (*torrent.Torrent, error) {
    t := & Torrent {
        TrackerNodes: trackerNodes,
        ChunkSize: DEFAULT_CHUNK_SIZE,
        ChunkHashes: make(map[int]byte)}

    // Attempt to find the file with the given path.
    var file *os.File
    var fi os.FileInfo
    if file, err := os.Open(path); err != nil {
        // Failed to read the file at the given path.
        return nil, err
    }
    if fi, err := file.Stat(); err != nil {
        // Failed to get information about the file.
        return nil, err
    }
    t.FileSize = int(fi.Size())

    // Read the file's contents.
    bytes := make([]byte, t.FileSize)
    if _, err := file.Read(bytes); err != nil {
        // Failed to read file contents.
        return nil, err
    }

    // Hash the entire file we just read.
    // Use this to determine the Torrent's ID.
    h := sha1.New()
    h.Write(bytes)
    t.ID = ID {Name: name, Hash: h.Sum(nil)}

    // Record hashes for every chunk.
    for chunkNum := 0; chunkNum < t.NumChunks(); chunkNum++ {
        if chunk, err := t.GetChunk(file, chunkNum); err != nil {
            return nil, err
        } else {
            h.Reset()
            h.Write(chunk)
            t.ChunkHashes[chunkNum] = h.Sum(nil)
        }
    }

    // Attempt to create an entry for this torrent on the trackers.
    if err := t.CreateEntry(); err != nil {
        return nil, err
    } else {
        return t, nil
    }
}

// CreateEntry attempts to create an entry for this Torrent on the Tracker
// nodes that it list.
//
// TODO: should this return info about what the error was?...or otherwise help
// us to recover in the case that we have the wrong list of tracker nodes?
func (t *torrent.Torrent) CreateEntry() error {
    // Attempt to contact one of the tracker nodes and create an entry for this
    // ID.
    //
    // TODO: only being able to create a torrent with a given ID once is fairly fragile
    // ...if we somehow forget that we created the torrent, our guarantees break... and
    // what if we do create on the tracker, but the client doesn't find out...would it
    // be blocked from ever creating a torrent with the name it wants to?
    for _, trackerNode := range t.trackerNodes {
        if conn, err := rpc.DialHTTP("tcp", trackerNode.HostPort); err == nil {
            // We found a live node in the tracker cluster.
            args := & trackerrpc.CreateArgs {Torrent: t}
            reply :=  & trackerrpc.UpdateReply {}
            if err := conn.Call("Tracker.CreateEntry", args, reply); err == nil {
                // Tracker responded to CreateEntry call. If create was successful,
                // continue creating Torrent. Otherwise, return an error.
                switch reply.Status {
                case trackerrpc.OK: 
                    // Successfully created Torrent on Tracker. Return it.
                    return nil

                case trackerrpc.InvalidID:
                    // Could not create Torrent on Tracker, because name/hash
                    // lead to an invalid ID. Recommend a name change.
                    return errors.New("Invalid name")

                case trackerrpc.InvalidTrackers:
                    // Could not create Torrent on Tracker, because given
                    // tracker nodes do not form a cluster.

                    // TODO: what should we do here? retry with correct trackers?
                    return errors.New("Invalid trackers")
                }
            }
        }
    }

    // No trackers responded. Cannot create Torrent.
    return errors.New("Trackers unresponsive")
}

// NumChunks returns the number of chunks into which we this Torrent's file is
// divided.
func (t *torrent.Torrent) NumChunks() int {
    if t.FileSize % t.ChunkSize == 0 {
        return t.FileSize / t.ChunkSize
    } else {
        // Round up.
        return (t.FileSize / t.ChunkSize) + 1
    }
}

// GetChunk returns a the chunk with the given number from this Torrent.
// If the given number is out of range, it returns a non-nil error.
func (t *torrent.Torrent) GetChunk(file *os.File, chunkNum int) ([]byte, error) {
    if start, length, err := t.getChunkBounds(chunkNum); err != nil {
        // Bad chunk number.
        return nil, err
    } else {
        bytes := make([]byte, length)
        if bytesRead, err := file.ReadAt(bytes, start); err != nil {
            // Read failed.
            return nil, err
        } else if bytesRead != length {
            // Read wrong number of bytes.
            return nil, errors.New("Read wrong number of bytes")
        } else {
            return bytes, nil
        }
    }
}

// SetChunk writes the given chunk at the position for the given chunk number
// in the given file.
// It returns a non-nil error if the write fails.
func (t *torrent.Torrent) SetChunk(file *os.File, chunkNum int, chunk []byte) error {
    var start, length int
    if start, length, err := t.getChunkBounds(chunkNum); err != nil {
        // Bad chunk number.
        return err
    }

    // If the file is not big enough for ths requested chunk, but the
    // torrent is big enough, attempt to extend the file.
    end := start + length
    if fi, err := file.Stat(); err != nil {
        // Failed to get file info.
        return err
    } else if fi.Size() < end {
        if err := file.Truncate(end); err != nil {
            // Failed to extend file.
            return err
        }
    }

    // Attempt to write to file.
    if bytesWritten, err := file.WriteAt(chunk, start); err != nil {
        // Could not write to file.
        return err
    } else if bytesWritten != length {
        // Wrote wrong number of bytes.
        return errors.New("Wrote wrong number of bytes")
    } else {
        // Write successful.
        return nil
    }
}

// ToFile serializes a torrent and writes it out to the given file.
func (t *torrent.Torrent) ToFile(path string) error {
    if bytes, err := json.Marshal(t); err != nil {
        return err
    } else if err := ioutil.WriteFile(path, bytes, MODE); err != nil {
        return err
    } else {
        // Successfully wrote Torrent to file.
        return nil
    }
}

// FromFile creates a Torrent from the file at the given path.
// This assumes that the Torrent at the given path was created using ToFile.
func FromFile(path string) (*torrent.Torrent, err) {
    var t torrent.Torrent
    if bytes, err := ioutil.ReadFile(path); err != nil {
        return nil, err
    } else if err := json.Unmarshal(bytes, &t); err != nil {
        return nil, err
    } else {
        // Successfully created Torrent from file.
        return &t, nil
    }
}

// getChunkBounds returns the start and length of the given chunk.
// Returns a non-nil error if the chunk number is invalid for this Torrent.
func (t *torrent.Torrent) getChunkBounds(chunkNum int) (int, int, error) {
    var start, length int
    start = chunkNum * t.ChunkSize
    if t.ChunkSize < t.FileSize - start {
        length = t.ChunkSize
    } else {
        length = t.FileSize - start
    }

    // Determine whether we're out of bounds.
    if start < 0 || t.FileSize <= start {
        return 0, 0, errors.New("Cannot get chunk: bad chunk number")
    } else {
        return start, length, nil
    }
}

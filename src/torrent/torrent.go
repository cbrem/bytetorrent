// This file contains utility funtions for creating and manipulating Torrents.

package torrent

import (
    "crypto/sha1"
    "encoding/gob"
    "errors"
    "fmt"
    "net/rpc"
    "os"
    "strings"

    "tracker/trackerproto"
    "torrent/torrentproto"
)

const (
    DEFAULT_CHUNK_SIZE int = 1000000 // Number of bytes per chunk
    MODE os.FileMode = 644 // Mode for writing torrent files
)

// New creates a new Torrent for the file at the given path.
// Gives this Torrent the given human-readable name.
// Throws an error if no file exists at this path
func New(path string, name string, trackerNodes []torrentproto.TrackerNode) (torrentproto.Torrent, error) {
    t := torrentproto.Torrent {
        TrackerNodes: trackerNodes,
        ChunkSize: DEFAULT_CHUNK_SIZE,
        ChunkHashes: make(map[int]string)}

    // Attempt to find the file with the given path.
    file, err := os.Open(path)
    if err != nil {
        // Failed to read the file at the given path.
        return torrentproto.Torrent{}, err
    }
    fi, err := file.Stat()
    if err != nil {
        // Failed to get information about the file.
        return torrentproto.Torrent{}, err
    }
    t.FileSize = int(fi.Size())

    // Read the file's contents.
    bytes := make([]byte, t.FileSize)
    if _, err := file.Read(bytes); err != nil {
        // Failed to read file contents.
        return torrentproto.Torrent{}, err
    }

    // Hash the entire file we just read.
    // Use this to determine the Torrent's ID.
    h := sha1.New()
    h.Write(bytes)
    t.ID = torrentproto.ID {Name: name, Hash: string(h.Sum(nil))}

    // Record hashes for every chunk.
    for chunkNum := 0; chunkNum < NumChunks(t); chunkNum++ {
        if chunk, err := ReadChunk(t, file, chunkNum); err != nil {
            return torrentproto.Torrent{}, err
        } else {
            h.Reset()
            h.Write(chunk)
            t.ChunkHashes[chunkNum] = string(h.Sum(nil))
        }
    }

    // Successfully created Torrent. Return it to user.
    // Note that this Torrent cannot be used until it is registered with the
    // Trackers using Register.
    return t, nil
}

// Load loads a serialized Torrent from the file at the given path.
// This assumes that the Torrent at the given path was created using ToFile.
func Load(path string) (torrentproto.Torrent, error) {
    var t torrentproto.Torrent
    if file, err := os.Open(path); err != nil {
        return torrentproto.Torrent{}, err
    } else if err := gob.NewDecoder(file).Decode(&t); err != nil {
        return torrentproto.Torrent{}, err
    } else {
        // Successfully created Torrent from file.
        return t, nil   
    }
}

// Save serializes a torrent and writes it out to the given file.
func Save(t torrentproto.Torrent, path string) error {
    if file, err := os.Create(path); err != nil {
        return err
    } else if err := gob.NewEncoder(file).Encode(t); err != nil {
        return err
    } else {
        // Successfully wrote Torrent to file.
        return nil
    }
}

// Register attempts to create an entry for this Torrent on the Tracker
// nodes that it list.
// Register should be called exactly once for a newly created Torrent (i.e.
// a Torrent corresponding to a new file, created with NewTorrent).
// Register should not be called for Torrents created by deserializing existing
// Torrents (using TorrentFromFile).
//
// Assuming that the torrent provided has never been registered before:
//   * If this method returns a non-nil error, then the Torrent is invalid
//     and cannot be used.
//   * Otherwise, the Torrent is valid and can be used. Additionally, its
//     ID is uniquely tied to the Torrent on the Tracker with which it is
//     registered.
func Register(t torrentproto.Torrent) error {
    // Attempt to contact one of the tracker nodes and create an entry for this
    // ID.
    for _, trackerNode := range t.TrackerNodes {
        if conn, err := rpc.DialHTTP("tcp", trackerNode.HostPort); err == nil {
            // We found a live node in the tracker cluster.
            args := & trackerproto.CreateArgs {Torrent: t}
            reply :=  & trackerproto.UpdateReply {}
            if err := conn.Call("RemoteTracker.CreateEntry", args, reply); err == nil {
                // Tracker responded to CreateEntry call. If create was successful,
                // continue creating Torrent. Otherwise, return an error.
                switch reply.Status {
                case trackerproto.OK: 
                    // Successfully created Torrent on Tracker. Return it.
                    return nil

                case trackerproto.InvalidID:
                    // Could not create Torrent on Tracker, because name/hash
                    // lead to an invalid ID. Recommend a name change.
                    return errors.New("Invalid name")

                case trackerproto.InvalidTrackers:
                    // Could not create Torrent on Tracker, because given
                    // tracker nodes do not form a cluster.
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
func NumChunks(t torrentproto.Torrent) int {
    if t.FileSize % t.ChunkSize == 0 {
        return t.FileSize / t.ChunkSize
    } else {
        // Round up.
        return (t.FileSize / t.ChunkSize) + 1
    }
}

// ReadChunk returns the chunk with the given number from this Torrent.
// If the given number is out of range, it returns a non-nil error.
func ReadChunk(t torrentproto.Torrent, file *os.File, chunkNum int) ([]byte, error) {
    if start, length, err := ChunkBounds(t, chunkNum); err != nil {
        // Bad chunk number.
        return nil, err
    } else {
        bytes := make([]byte, length)
        if bytesRead, err := file.ReadAt(bytes, int64(start)); err != nil {
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

// WriteChunk writes the given chunk at the position for the given chunk number
// in the given file.
// It returns a non-nil error if the write fails.
func WriteChunk(t torrentproto.Torrent, file *os.File, chunkNum int, chunk []byte) error {
    start, length, err := ChunkBounds(t, chunkNum)
    if err != nil {
        // Bad chunk number.
        return err
    }

    // If the file is not big enough for ths requested chunk, but the
    // torrent is big enough, attempt to extend the file.
    end := start + length
    if fi, err := file.Stat(); err != nil {
        // Failed to get file info.
        return err
    } else if fi.Size() < int64(end) {
        if err := file.Truncate(int64(end)); err != nil {
            // Failed to extend file.
            return err
        }
    }

    // Attempt to write to file.
    if bytesWritten, err := file.WriteAt(chunk, int64(start)); err != nil {
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

// ChunkBounds returns the start and length of the given chunk.
// Returns a non-nil error if the chunk number is invalid for this Torrent.
func ChunkBounds(t torrentproto.Torrent, chunkNum int) (int, int, error) {
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

// String converts the given torrent to a human-readable string representation.
func String(t torrentproto.Torrent) string {
    fields := make([]string, 0)
    fields = append(fields, fmt.Sprintf("ID: {Name: %s, Hash: %s}", t.ID.Name, t.ID.Hash))
    fields = append(fields, fmt.Sprintf("File Size: %d", t.FileSize))
    fields = append(fields, fmt.Sprintf("Chunk Size: %d", t.ChunkSize))

    chunkHashes := make([]string, 0)
    chunkHashes = append(chunkHashes, "Chunk Hashes")
    for chunkNum, hash := range t.ChunkHashes {
        chunkHashes = append(chunkHashes, fmt.Sprintf("%d: %s", chunkNum, hash))
    }
    fields = append(fields, strings.Join(chunkHashes, "\n\t"))

    trackerNodes := make([]string, 0)
    trackerNodes = append(trackerNodes, "Tracker IPs")
    for _, trackerNode := range t.TrackerNodes {
        trackerNodes = append(trackerNodes, trackerNode.HostPort)
    }
    fields = append(fields, strings.Join(trackerNodes, "\n\t"))    

    return strings.Join(fields, "\n")
}

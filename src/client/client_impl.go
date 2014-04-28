// TODO:
//  - cache chunks on the client?
//  - update lfl when things happen
//  - make sure that this works with the whole torrent-validating sheme
//  - don't just send chunkID in rpc... wrap this..also, make sure
//    clientrpc is using the right thing

package client

import (
    "net/rpc"
    "os"
    "rand"

    "src/clientrpc"
    "src/torrent"
)

// The client's representation of a request to get a chunk.
type Get struct {
    *torrent.ChunkID,
    Reply chan *clientrpc.GetArgs
}

// The client's representation of a request to close the client.
type Close struct {
    // The client passes back any error involved with closing on this channel.
    Reply chan error
}

// The client's representation of a request to offer a file to a Tracker.
type Offer struct {
    // A Torrent for the file being offered.
    Torrent *torrent.Torrent

    // The local path to the file being offered.
    Path string

    // The client passes back any error involved with offering on this channel.
    Reply chan error
}

// The client's representation of a request to download a file.
type Download struct {
    // A Torrent for the file to download.
    Torrent *torrent.Torrent

    // The local path to the location to which the file should download.
    Path string
    
    // The client passes back any error involved with downloading on this channel.
    Reply chan error
}

// A ByteTorrent Client implementation.
type client struct {
    // A map from Torrent IDs to associated local file states
    localFiles map[string]*LocalFile

    // Requests to get chunks from this client.
    gets chan *Get

    // Push to this channel to request that the client close.
    closes chan *Close

    // Push to this channel to request that the client downloads files.
    downloads chan *Download

    // Push to this channel to request that the client offer a file.
    offers chan *Offer

    // Go routines pass the IDs of successfully downloaded chunks to the
    // eventHandler via this channel.
    downloadedChunks chan *torrent.ChunkID

    // Go routines pass the IDs of missing chunks to the eventHandler via this
    // channel.
    missingChunks chan *torrent.ChunkID

    // This client's hostport.
    // TODO: do we need both? or just addr?
    hostPort string
}

// New creates and starts a new ByteTorrent Client.
func New(localFiles map[string]*LocalFile, lfl LocalFileListener, hostPort string) (Client, error) {
    c := & {
        localFiles: localFiles,
        lfl: lfl,
        gets: make(chan *Get),
        closes: make(chan *Close),
        offers: make(chan *Offer),
        downloads: make(chan *Download),
        hostPort: hostPort}

    // Configure this Client to receive RPCs on RemoteClient at hostPort.
    if ln, err := net.Listen("tcp", hostPort); err != nil {
        // Failed to listen on the given host:port.
        return nil, err
    } else if err := rpc.Register(clientrpc.Wrap(c)); err != nil {
        // Failed to register this Client for RPCs as a RemoteClient.
        return nil, err
    } else {
        // Successfully registered to receive RPCs.
        // Handle these RPCs and other Client events.
        // Return the started Client.
        rpc.HandleHTTP()
        go http.Serve(ln, nil)
        go.eventHandler()
        return c, nil
    }
}

func (c *client) GetChunk(args *torrent.ChunkID, reply *clientrpc.GetReply) error {
    replyChan := make(chan *clientrpc.GetReply)
    get := &Get{
        ChunkID: args,
        Reply: replyChan}
    c.gets <- get
    *reply = *(<-replyChan)
    return nil
}

func (c *client) OfferFile(t *torrent.Torrent, path string) error {
    replyChan := make(chan error)
    offer := & Offer {
        Torrent: t,
        Path: path,
        Reply: replyChan}
    c.offers <- offer
    return <- replyChan
}

func (c *client) DownloadFile(*torrent.Torrent, path string) error {
    replyChan := make(chan error)
    download := & Download {
        Torrent: t,
        Path: path,
        Reply: replyChan}
    c.downloads <- download
    return <-replyChan
}

func (c *client) Close() error {
    replyChan := make(chan error)
    cl := & Close {
        Reply: replyChan}
    c.closes <- cl
    return <-replyChan
}

// eventHandler synchronizes all events on this Client.
func (c *client) eventHandler() {
    for {
        select {

        // The user has supplied a torrent and requested a download.
        // Service the download asynchronously, and respond to the user
        // when done.
        // The IDs of successfully downloaded chunks will be passed back to
        // the eventHandler as they arrive.
        case download := <- c.downloads:
            go downloadFile(download, c.downloadedChunks)

        // Another Client has requested a chunk.
        case get := <- c.gets:
            if localFile, ok := c.localFiles[get.ChunkID.ID]; !ok {
                // This Client does not know about a local file which
                // corresponds to the requested Torrent ID.
                get.Reply <- []
            } else if _, ok := localFile[get.ChunkID.ChunkNum]; !ok {
                // This Client knows about the requested file,
                // but does not have the requested chunk.
                get.Reply <- []
            } else if file, err := os.Open(localFile.Path); err != nil {
                // The Client thought that it had the requested chunk,
                // but cannot open the file containing the chunk.
                get.Reply <- []
            } else if chunk, err := torrent.GetChunk(file, chunkNum); err != nil {
                // The Client could not get the requested chunk from the file.
                get.Reply <- []
            } else {
                // Got the requested chunk. Send it back to the requesting
                // client.
                get.Reply <- chunk
            }

        // Close the client.
        case cl := <- c.closes:
            return

        // The user wants to offer a file to a Tracker.
        // Record on the Client that this file is available.
        // Then, inform the relevant Tracker.
        case offer := <- c.offers:
            // Record that this client has these chunks.
            // Note that we do not check a chunk's hash here to see if it
            // is valid. This is a task for the Client receiving the chunk.
            //
            // TODO: check that we do not re-offer, in case we already have the chunk.
            localFile := & LocalFile {
                Torrent: offer.Torrent,
                Path: offer.Path,
                Chunks: make(map[int]struct{})}
            c.localFiles[offer.Torrent.ID] = localFile
            for chunkID := 0; chunkID < offer.Torrent.NumChunks(); chunkID++ {
                localFile[i] = struct{}{}
            }

            // Inform this Client's LocalFileListener that local files have
            // been updated.
            c.lfl.OnChange(& LocalFileChange {
                LocalFile: localFile,
                LocalFileOperation: LocalFileUpdate})

            // Offer this file to a Tracker.
            if trackerConn, err := getResponsiveTrackerNode(t); err != nil {
                // Unable to get a responsive Tracker node.
                offer.Reply <- nil
                return
            } else {
                // Confirm to the Tracker that this client has all chunks associated with
                // the Torrent.
                for chunkNum := 0; chunkNum < t.NumChunks(); chunkNum++ {
                    args := & trackerrpc.ConfirmArgs{
                        ID: t.ID,
                        ChunkNum: chunkNum,
                        Addr: hostPort}
                    reply := & trackerrpc.UpdateReply{}
                    if err := rpc.Call("Tracker.ConfirmChunk", args, reply); err != nil {
                        // Previously responsive Tracker has failed.
                        offer.Reply <- err
                        return
                    }
                    if reply.Status == trackerrpc.FileNotFound {
                        // Torrent refers to a file which does not exist on the Tracker.
                        offer.Reply <- errors.New("Tried to offer file which does not exist on Tracker")
                        return
                    }
                }
            }

            // Inform the user that this offer completed without error.
            offer.Reply <- nil

        // Record that a chunk is not available on this client.
        case missing := <- c.missingChunks:
            // Record locally that the client does not have this chunk.
            // Report to the Tracker that the client does not have this chunk.

            // TODO: currently doesn't do anything, because there's a design problem!
            // this Client can't self-report, because it doesn't know what Tracker to
            // report to. And it can't know this tracker unless the Client that
            // requested the chunk passes that Torrent...or we somehow keep a record
            // locally of which Trackers think that this Client has this chunk

        // Record that this client has this chunk.
        // Note that we do not check the chunk's hash here to see if it
        // is valid. This is a task for the Client receiving the chunk.
        case chunkID := <- c.downloadedChunks:
            // Record that this client has this chunk.
            var localFile *LocalFile
            if localFile, ok := c.localFiles[chunkID.ID]; !ok {
                // There is no entry for this torrent ID. Create one.
                localFile = & LocalFile {
                    Torrent: offer.Torrent,
                    Path: offer.Path,
                    Chunks: make(map[int]struct{})}
                c.localFiles[chunkID.ID] = localFile

                // Inform this Client's LocalFileListener that local files have
                // been added.
                c.lfl.OnChange(& LocalFileChange {
                    LocalFile: localFile,
                    LocalFileOperation: LocalFileAdd})
            }
            localFile.Chunks[chunkID.chunkNum] = struct{}{}

            // Inform this Client's LocalFileListener that local files have
            // been updated.
            c.lfl.OnChange(& LocalFileChange {
                LocalFile: localFile,
                LocalFileOperation: LocalFileUpdate})
        }
    }
}

// getResponsiveTrackerNode gets a live connection to a Tracker node.
// However, there is no guarantee that this connection won't die immediately.
func getResponsiveTrackerNode(t *torrent.Torrent) error {
    for _, trackerNode := range t.trackerNodes {
        if conn, err := rpc.DialHTTP("tcp", trackerNode.HostPort); err == nil {
            // Found a live node.
            return conn, nil;
        }
    }

    // Didn't find any live nodes on one pass.
    return nil, errors.New("Could not find a responsive Tracker")
}

// downloadFile gets all chunks of a file from Clients which have them.
// If the chunk is not available, sends a non-nil error to the user.
// As the chunks are downloaded, it informs the Client that they have arrived
// and offers them to the Tracker.
//
// TODO: do we have a function for offering just a chunk?
// TODO: should this return an error if the chunks aren't available within some time?
func downloadFile(download *Download, downloadChunks chan *Downloaded) {
    // Create a file to hold this chunk.
    var file *os.File
    if file, err := os.Create(download.Path); err != nil {
        // Failed to create file at given path.
        download.Reply <- err
        return
    }

    // Download the chunks associated with this file in a random order.
    for chunkNum := range rand.Perm(download.Torrent.NumChunks()) {
        // Get list of peers with chunk.
        var peers []string
        chunkID := torrent.ChunkID {
            ID: download.Torrent.ID,
            ChunkNum: chunkNum}
        trackerArgs := & trackerrpc.RequestArgs {Chunk: chunkID}
        trackerReply := & trackerrpc.RequestReply {}
        if trackerConn, err := getResponsiveTrackerNode(t); err != nil {
            // Could not contact a tracker.
            download.Reply <- err
            return
        } else if err := rpc.Call("Tracker.RequestChunk", trackerArgs, trackerReply); err != nil {
            // Failed to make RPC.
            download.Reply <- err
            return
        } else if err := downloadChunk(download, file, chunkNum, trackerReply.Peers); err != nil {
            // Failed to download this chunk.
            download.Reply <- err
            return
        } else {
            // Successfully downloaded and wrote this chunk.
            // Inform the Client.
            downloadedChunks <- chunkID
        }
    }

    // Successfully downloaded and wrote all chunks.
    download.Reply <- nil
}

// downloadChunk attemps to download and locally write one chunk.
// If it fails, it returns a non-nil error.
func downloadChunk(download *Download, file *os.File, chunkNum int, peers []string) err {
    // Try peers until one responds with chunk.
    //
    // TODO: maybe add timeouts so we don't get hung up on any peer?
    peerArgs := & clientrpc.GetArgs{
        ChunkID: torrent.ChunkID {
            ID: download.Torrent.ID,
            ChunkNum: chunkNum}}
    peerReply := & clientrpc.GetReply{}
    h := sha1.New()
    for _, hostPort := range peers {
        if peer, err := rpc.DialHTTP("tcp", hostPort); err != nil {
            // Failed to connect.
            continue
        } else if err := peer.Call("Client.GetChunk", peerArgs, peerReply); err != nil {
            // Failed to make RPC.
            continue
        }

        chunk := peerReply.Chunk
        h.Reset()
        h.Write(chunk)
        if h.Sum(nil) != download.Torrent.ChunkHashes[chunkNum] {
            // Chunk had bad hash.
            continue
        } else if err := download.Torrent.SetChunk(file, chunkNum, chunk) {
            // Failed to write chunk locally.
            continue
        } else {
            // Successfully downloaded and wrote chunk.
            return nil
        }
    }

    // Failed to get the chunk from a peer.
    return errors.New("No peers responded with chunk")
}

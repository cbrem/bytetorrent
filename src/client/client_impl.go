// TODO: cache chunks on the client?

package client

// The client's representation of a request to get a chunk.
type Get struct {
    Args *clientrpc.GetArgs,
    Reply chan *clientrpc.GetArgs
}

// The client's representation of a request to close the client.
type Close struct {
    // The client passes back any error involved with closing on this channel.
    Reply chan error
}

// The client's representation of a request to offer a file to a Tracker.
type Offer struct {
    // The client passes back any error involved with offering on this channel.
    Reply chan error
}

// The client's representation of a request to download a file.
type Download struct {
    // The client passes back any error involved with downloading on this channel.
    Reply chan error
}

// The client's representation of a seedable file.
type Seedable struct {
    Torrent *torrent.Torrent
    Path string // Path to a local copy of the file
    Chunks []bool // Indicates whether this client possesses each chunk.
}

// A ByteTorrent Client implementation.
type client struct {
    // A map from Torrent IDs to associated Seedables.
    files map[string]*Seedable

    // The path at which this client will save data.
    savePath string

    // Requests to get chunks from this client.
    gets chan *Get

    // Push to this channel to request that the client close.
    closes chan *Close

    // Push to this channel to request that the client downloads files.
    downloads chan *Download

    // Push to this channel to request that the client offer a file.
    offers chan *Offer

    // This client's hostport.
    // TODO: do we need both? or just addr?
    hostPort string
}

// New creates and starts a new ByteTorrent Client.
// This file will save its state in a file at the given path.
// TODO: will we ever return an error?
func New(string savePath) (Client, error) {
    c := & {
        file: make(map[string]*Seedable),
        savePath: savePath,
        gets: make(chan *Get),
        closes: make(chan *Close),
        offers: make(chan *Offer),
        downloads: make(chan *Download),
        hostPort: /* TODO */}

    go c.eventHandler()

    return (c, nil)
}

func (c *client) GetChunk(args *clientrpc.GetArgs, reply *clientrpc.GetReply) error {
    replyChan := make(chan *clientrpc.GetReply)
    get := &Get{
        Args:  args,
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
    c.closes <- download
    return <-replyChan
}

func (c *client) Close() error {
    replyChan := make(chan error)
    cl := & Close {
        Reply: replyChan}
    c.closes <- cl
    return <-replyChan
}

// Gets a live connection to a Tracker node.
// However, there is no guarantee that this connection won't die immediately.
func (c *client) getResponsiveTrackerNode(t *torrent.Torrent) error {
    for _, trackerNode := range t.trackerNodes {
        if conn, err := rpc.DialHTTP("tcp", trackerNode.HostPort); err == nil {
            // Found a live node.
            return (conn, nil);
        }
    }

    // Didn't find any live nodes on one pass.
    return nil, errors.New("Could not find a responsive Tracker")
}

// Offers a file from within eventHandler.
func (c *client) offerFile(t *torrent.Torrent) error {
    var trackerConn *rpc.Client
    if trackerConn, err := c.getResponsiveTrackerNode(t); err != nil {
        // Unable to get a responsive Tracker node.
        return err
    }

    // Confirm to the Tracker that this client has all chunks associated with
    // the Torrent.
    for chunkNum := 0; chunkNum < t.NumChunks(); chunkNum++ {
        args := & trackerrpc.ConfirmArgs{
            ID: t.ID,
            ChunkNum: chunkNum,
            Addr: c.hostPort}
        reply := & trackerrpc.UpdateReply{}
        if err := rpc.Call("Tracker.ConfirmChunk", args, reply); err != nil {
            // Previously responsive Tracker has failed.
            return err
        }
        if reply.Status == trackerrpc.FileNotFound {
            // Torrent refers to a file which does not exist on the Tracker.
            return errors.New("Tried to offer file which does not exist on Tracker")
        }
    }

    // Successfully informed Tracker that this client has all chunks of file.
    return nil
}

// Saves this client's state out to file.
func (c *client) saveState() {
    // TODO
}

// Routes all events on this Client.
func (c *client) eventHandler() {
    for {
        select {
        case download := <- c.downloads:
            // TODO: spawn a bucnh of threads to do this asynchronously...but reply to things when done

        case get := <- c.gets:
            // TODO

        case cl := <- c.closes:
            c.saveState()
            return

        case offer := <- c.offers:
            offer.Reply <- c.offerFile()
        }
    }
}

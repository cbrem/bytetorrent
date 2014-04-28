// A simple implementation of tracker.Tracker for testing only.
// Client and Torrent implementations can use Tracker instead of the real
// Tracker for tests which only wish to examine Client/Torrent functionality.
// This implementation uses a singer node instead of a Paxos cluster.

package dummytracker

type Request struct {
    Args  *trackerrpc.RequestArgs
    Reply chan *trackerrpc.RequestReply
}

type Confirm struct {
    Args  *trackerrpc.ConfirmArgs
    Reply chan *trackerrpc.UpdateReply
}

type Report struct {
    Args  *trackerrpc.ReportArgs
    Reply chan *trackerrpc.UpdateReply
}

type Create struct {
    Args  *trackerrpc.CreateArgs
    Reply chan *trackerrpc.UpdateReply
}

type GetTrackers struct {
    Args  *trackerrpc.TrackersArgs
    Reply chan *trackerrpc.TrackersReply
}

type testTracker struct {
    // Set-up
    hostPort    string

    // Channels for rpc calls
    requests    chan *Request
    confirms    chan *Confirm
    reports     chan *Report
    creates     chan *Create
    getTrackers chan *GetTrackers

    // Actual data storage
    torrents   map[torrent.ID]torrent.Torrent              // Map the torrentID to the Torrent information
    peers      map[torrent.ChunkID](map[string](struct{})) // Maps chunk info -> list of host:port with that chunk
}

func NewTrackerServer(hostPort string) (trackerrpc.Tracker, error) {
    t := &trackerServer{
        hostPort:             hostPort,
        registers:            make(chan *Register),
        reports:              make(chan *Report),
        requests:             make(chan *Request),
        creates:              make(chan *Create),
        getTrackers:          make(chan *GetTrackers),
        peers:                make(map[string](map[string](struct{}))),
        trackers:             make([]*rpc.Client, numNodes)}

    // Attempt to service connections on the given port.
    // Then, configure this TrackerServer to receive RPCs over HTTP on a
    // trackerrpc.Tracker interface.
    if ln, lnErr := net.Listen("tcp", hostPort); lnErr != nil {
        return nil, lnErr
    } else if regErr := rpc.Register(t); regErr != nil {
        return nil, regErr
    } else {
        rpc.HandleHTTP()
        go http.Serve(ln, nil)

        // Start this TrackerServer's eventHandler, which will respond to RPCs,
        // and return it.
        go t.eventHandler()
        return t, nil
    }
}

func (t *trackerServer) ReportMissing(args *trackerrpc.ReportArgs, reply *trackerrpc.UpdateReply) error {
    replyChan := make(chan *trackerrpc.UpdateReply)
    report := &Report{
        Args:  args,
        Reply: replyChan}
    t.reports <- report
    *reply = *(<-replyChan)
    return nil
}

func (t *trackerServer) ConfirmChunk(args *trackerrpc.ConfirmArgs, reply *trackerrpc.UpdateReply) error {
    replyChan := make(chan *trackerrpc.UpdateReply)
    confirm := &Confirm{
        Args:  args,
        Reply: replyChan}
    t.confirms <- confirm
    *reply = *(<-replyChan)
    return nil
}

func (t *trackerServer) CreateEntry(args *trackerrpc.CreateArgs, reply *trackerrpc.UpdateReply) error {
    replyChan := make(chan *trackerrpc.UpdateReply)
    create := &Create{
        Args:  args,
        Reply: replyChan}
    t.creates <- create
    *reply = *(<-replyChan)
    return nil
}

func (t *trackerServer) RequestChunk(args *trackerrpc.RequestArgs, reply *trackerrpc.RequestReply) error {
    replyChan := make(chan *trackerrpc.RequestReply)
    request := &Request{
        Args:  args,
        Reply: replyChan}
    t.requests <- request
    *reply = *(<-replyChan)
    return nil
}

func (t *trackerServer) GetTrackers(args *trackerrpc.TrackersArgs, reply *trackerrpc.TrackersReply) error {
    replyChan := make(chan *trackerrpc.TrackersReply)
    trackers := &GetTrackers{
        Args:  args,
        Reply: replyChan}
    t.getTrackers <- trackers
    *trackers = *(<-replyChan)
    return nil
}

func (t *trackerServer) eventHandler() {
    for {
        select {
        case rep := <-t.reports:
            // A client has reported that it does not have a chunk
            if tor, ok := t.torrents[rep.Args.Chunk.ID]; !ok {
                // File does not exist
                rep.Reply <- &trackerrpc.UpdateReply{Status: trackerrpc.FileNotfound}
            } else if req.Args.Chunk.ChunkNum < 0 || req.Args.Chunk.ChunkNum >= tor.NumChunks() {
                // ChunkNum is not right for this file
                rep.Reply <- &trackerrpc.UpdateReply{Status: trackerrpc.OutOfRange}
            } else {
                // Remove this torrent from the client's record.
                delete(t.peers[rep.Args.Chunk], rep.Args.HostPort)
            }
        case conf := <-t.confirms:
            // A client has confirmed that it has a chunk
            if tor, ok := t.torrents[conf.Args.Chunk.ID]; !ok {
                // File does not exist
                conf.Reply <- &trackerrpc.UpdateReply{Status: trackerrpc.FileNotfound}
            } else if conf.Args.Chunk.ChunkNum < 0 || conf.Args.Chun.ChunkNum >= tor.NumChunks() {
                // ChunkNum is not right for this file
                conf.Reply <- &trackerrpc.UpdateReply{Status: trackerrpc.OutOfRange}
            } else {
                // Mark that the client has this chunk, creating the map for this
                // chunk if necessary.
                if _, ok := t.peers[conf.Args.ChunkID]; !ok {
                    t.peers.[conf.Args.ChunkID] = make(map[string]struct{})
                }
                t.peers[conf.Args.ChunkID][conf.Args.HostPort] = struct{}{}
            }
        case cre := <-t.creates:
            // A client has requested to create a new file
            if tor, ok := t.tor[cre.Args.Torrent.ID]; !ok {
                // ID not in use, so add an entry for it.
                t.tor[cre.Args.Torrent.ID] = cre.Args.Torrent
            } else {
                // File already exists, so tell the client that this ID is invalid
                cre.Reply <- &trackerrpc.UpdateReply{Status: trackerrpc.InvalidID}
            }
        case req := <-t.requests:
            // A client has requested a list of users with a certain chunk
            if tor, ok := t.torrents[req.Args.Chunk.ID]; !ok {
                // File does not exist
                req.Reply <- &trackerrpc.RequestReply{Status: trackerrpc.FileNotfound}
            } else if req.Args.Chunk.ChunkNum < 0 || req.Args.Chunk.ChunkNum >= tor.NumChunks() {
                // ChunkNum is not right for this file
                req.Reply <- &trackerrpc.RequestReply{Status: trackerrpc.OutOfRange}
            } else {
                // Get a list of all peers, then respond
                peers := make([]string, 0)
                for k, _ := range t.peers[req.Args.Chunk] {
                    append(peers, k)
                }
                req.Reply <- &trackerrpc.RequestReply{
                    Status: trackerrpc.OK,
                    Peers:  peers}
            }
        case gt := <-t.getTrackers:
            // Reply with only this node's host:port.
            gt.Reply <- &trackerrpc.TrackersReply{
                Status:    trackerrpc.OK,
                HostPorts: []string{t.hostPort}}
        }
    }
}

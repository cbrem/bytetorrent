// A simple implementation of tracker.Tracker for testing only.
// Client and Torrent implementations can use Tracker instead of the real
// Tracker for tests which only wish to examine Client/Torrent functionality.
// This implementation uses a singer node instead of a Paxos cluster.

package dummytracker

import (
    "net"
    "net/http"
    "net/rpc"

    "torrent"
    "torrent/torrentproto"
    "tracker/trackerproto"
)

type Request struct {
    Args  *trackerproto.RequestArgs
    Reply chan *trackerproto.RequestReply
}

type Confirm struct {
    Args  *trackerproto.ConfirmArgs
    Reply chan *trackerproto.UpdateReply
}

type Report struct {
    Args  *trackerproto.ReportArgs
    Reply chan *trackerproto.UpdateReply
}

type Create struct {
    Args  *trackerproto.CreateArgs
    Reply chan *trackerproto.UpdateReply
}

type GetTrackers struct {
    Args  *trackerproto.TrackersArgs
    Reply chan *trackerproto.TrackersReply
}

type dummyTracker struct {
    // Set-up
    hostPort    string

    // Channels for rpc calls
    requests    chan *Request
    confirms    chan *Confirm
    reports     chan *Report
    creates     chan *Create
    getTrackers chan *GetTrackers

    // Actual data storage
    torrents   map[torrentproto.ID]torrentproto.Torrent              // Map the torrentID to the Torrent information
    peers      map[torrentproto.ChunkID](map[string](struct{})) // Maps chunk info -> list of host:port with that chunk
}

func New(hostPort string) (DummyTracker, error) {
    dt := & dummyTracker{
        hostPort:             hostPort,
        requests:             make(chan *Request),
        confirms:             make(chan *Confirm),
        reports:              make(chan *Report),
        creates:              make(chan *Create),
        getTrackers:          make(chan *GetTrackers),
        torrents:             make(map[torrentproto.ID]torrentproto.Torrent),
        peers:                make(map[torrentproto.ChunkID](map[string](struct{})))}

    // Attempt to service connections on the given port.
    // Then, configure this TrackerServer to receive RPCs over HTTP on a
    // tracker.Tracker interface.
    //
    // TODO: update rpc.Register to use tracker.WrapRemote as soon as that
    // compiles
    if ln, lnErr := net.Listen("tcp", hostPort); lnErr != nil {
        return nil, lnErr
    } else if regErr := rpc.RegisterName("RemoteTracker", Wrap(dt)); regErr != nil {
        return nil, regErr
    } else {
        rpc.HandleHTTP()
        go http.Serve(ln, nil)

        // Start this TrackerServer's eventHandler, which will respond to RPCs,
        // and return it.
        go dt.eventHandler()
        return dt, nil
    }
}

func (dt *dummyTracker) ReportMissing(args *trackerproto.ReportArgs, reply *trackerproto.UpdateReply) error {
    replyChan := make(chan *trackerproto.UpdateReply)
    report := &Report{
        Args:  args,
        Reply: replyChan}
    dt.reports <- report
    *reply = *(<-replyChan)
    return nil
}

func (dt *dummyTracker) ConfirmChunk(args *trackerproto.ConfirmArgs, reply *trackerproto.UpdateReply) error {
    replyChan := make(chan *trackerproto.UpdateReply)
    confirm := &Confirm{
        Args:  args,
        Reply: replyChan}
    dt.confirms <- confirm
    *reply = *(<-replyChan)
    return nil
}

func (dt *dummyTracker) CreateEntry(args *trackerproto.CreateArgs, reply *trackerproto.UpdateReply) error {
    replyChan := make(chan *trackerproto.UpdateReply)
    create := &Create{
        Args:  args,
        Reply: replyChan}
    dt.creates <- create
    *reply = *(<-replyChan)
    return nil
}

func (dt *dummyTracker) RequestChunk(args *trackerproto.RequestArgs, reply *trackerproto.RequestReply) error {
    replyChan := make(chan *trackerproto.RequestReply)
    request := &Request{
        Args:  args,
        Reply: replyChan}
    dt.requests <- request
    *reply = *(<-replyChan)
    return nil
}

func (dt *dummyTracker) GetTrackers(args *trackerproto.TrackersArgs, reply *trackerproto.TrackersReply) error {
    replyChan := make(chan *trackerproto.TrackersReply)
    trackers := &GetTrackers{
        Args:  args,
        Reply: replyChan}
    dt.getTrackers <- trackers
    *reply = *(<-replyChan)
    return nil
}

func (dt *dummyTracker) eventHandler() {
    for {
        select {
        case rep := <-dt.reports:
            // A client has reported that it does not have a chunk
            if tor, ok := dt.torrents[rep.Args.Chunk.ID]; !ok {
                // File does not exist
                rep.Reply <- &trackerproto.UpdateReply{Status: trackerproto.FileNotFound}
            } else if rep.Args.Chunk.ChunkNum < 0 || rep.Args.Chunk.ChunkNum >= torrent.NumChunks(tor) {
                // ChunkNum is not right for this file
                rep.Reply <- &trackerproto.UpdateReply{Status: trackerproto.OutOfRange}
            } else {
                // Remove this torrent from the client's record.
                delete(dt.peers[rep.Args.Chunk], rep.Args.HostPort)
                rep.Reply <- &trackerproto.UpdateReply{Status: trackerproto.OK}
            }
        case conf := <-dt.confirms:
            // A client has confirmed that it has a chunk
            if tor, ok := dt.torrents[conf.Args.Chunk.ID]; !ok {
                // File does not exist
                conf.Reply <- &trackerproto.UpdateReply{Status: trackerproto.FileNotFound}
            } else if conf.Args.Chunk.ChunkNum < 0 || conf.Args.Chunk.ChunkNum >= torrent.NumChunks(tor) {
                // ChunkNum is not right for this file
                conf.Reply <- &trackerproto.UpdateReply{Status: trackerproto.OutOfRange}
            } else {
                // Mark that the client has this chunk, creating the map for this
                // chunk if necessary.
                if _, ok := dt.peers[conf.Args.Chunk]; !ok {
                    dt.peers[conf.Args.Chunk] = make(map[string]struct{})
                }
                dt.peers[conf.Args.Chunk][conf.Args.HostPort] = struct{}{}
                conf.Reply <- &trackerproto.UpdateReply{Status: trackerproto.OK}
            }
        case cre := <-dt.creates:
            // A client has requested to create a new file
            if _, ok := dt.torrents[cre.Args.Torrent.ID]; !ok {
                // ID not in use, so add an entry for it.
                dt.torrents[cre.Args.Torrent.ID] = cre.Args.Torrent
                cre.Reply <- &trackerproto.UpdateReply{Status: trackerproto.OK}
            } else {
                // File already exists, so tell the client that this ID is invalid
                cre.Reply <- &trackerproto.UpdateReply{Status: trackerproto.InvalidID}
            }
        case req := <-dt.requests:
            // A client has requested a list of users with a certain chunk
            if tor, ok := dt.torrents[req.Args.Chunk.ID]; !ok {
                // File does not exist
                req.Reply <- &trackerproto.RequestReply{Status: trackerproto.FileNotFound}
            } else if req.Args.Chunk.ChunkNum < 0 || req.Args.Chunk.ChunkNum >= torrent.NumChunks(tor) {
                // ChunkNum is not right for this file
                req.Reply <- &trackerproto.RequestReply{Status: trackerproto.OutOfRange}
            } else {
                // Get a list of all peers, then respond
                peers := make([]string, 0)
                for k, _ := range dt.peers[req.Args.Chunk] {
                    peers = append(peers, k)
                }
                req.Reply <- &trackerproto.RequestReply{
                    Status: trackerproto.OK,
                    Peers:  peers}
            }
        case gt := <-dt.getTrackers:
            // Reply with only this node's host:port.
            gt.Reply <- &trackerproto.TrackersReply{
                Status:    trackerproto.OK,
                HostPorts: []string{dt.hostPort}}
        }
    }
}

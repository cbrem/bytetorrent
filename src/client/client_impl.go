package client

// A ByteTorrent Client implementation.
type client struct {
    // TODO
}

// New creates and starts a new ByteTorrent Client.
func New() (Client, error) {
    // TODO
}

func (c *client) GetChunk(args *clientrpc.GetChunkArgs, reply *clientrpc.GetChunkReply) error {

}

func (c *client) OfferTorrent(t *torrent.Torrent, path string) error {

}

func (c *client) DownloadFile(*torrent.Torrent, path string) error {

}

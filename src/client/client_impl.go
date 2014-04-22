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
    // TODO
}

func (c *client) RegisterTorrent(t *torrent.Torrent, path string) error {
    // TODO
}

func (c *client) DownloadFile(t *torrent.Torrent, path string, done <-chan struct{}) error {
    // TODO
}

package clientrpc

// Clients will handle RPCs on this interface.
type RemoteClient struct {
    GetChunk(*GetChunkArgs, *GetChunkReply)
}

type Client struct {
    RemoteClient
}

// Returns a wrapped RemoteClient which is guaranteed to handle RPCs on only
// the puglic RemoteClient requests.
func Wrap(c RemoteClient) RemoteClient {
    return & Client{c}
}

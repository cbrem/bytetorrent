package client

import (
    "client/clientproto"
)

// Clients will handle RPCs on this interface.
type RemoteClient interface {
    GetChunk(*clientproto.GetArgs, *clientproto.GetReply) error
}

type WrappedClient struct {
    RemoteClient
}

// Returns a wrapped RemoteClient which is guaranteed to handle RPCs on only
// the puglic RemoteClient requests.
func Wrap(c RemoteClient) RemoteClient {
    return & WrappedClient{c}
}

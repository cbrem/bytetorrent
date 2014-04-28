package client

// Applications should implement this interface if they wish to know when the
// local files associated with a torrent Client change.
type LocalFileListener interface {
    // OnChange notifies a LocalFileListener when a local file changes.
    OnChange(*LocalFileChange)
}

package main

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "os"
    "strings"

    "torrent"
    "torrent/torrentproto"
    "client"
    "client/clientproto"
)

const (
    SAVE_PATH string = "state.txt" // File where we will save Client state on exit
    MODE os.FileMode = 644 // Mode for saving state to file
)

var (
    USAGE string = strings.Join([]string{
        "Usage:",
        "\t<program_name> <client host:port> <tracker 0 host:port> ... <tracker n-1 host:port>",
        ""}, "\n")
    COMMANDS string = strings.Join([]string{
        "Command:",
        "\tCREATE <file_path> <torrent_path> <name>",
        "\tOFFER <file_path> <torrent_path>",
        "\tDOWNLOAD <file_path> <torrent_path>",
        "\tEXIT",
        ""}, "\n")
)

// A listener which updates the view when the Client changes local files.
type clientFileListener struct {}

func (cfl *clientFileListener) OnChange(change *clientproto.LocalFileChange) {
    switch change.Operation {
    case clientproto.LocalFileAdd:
        fmt.Println("Added file:", changeToString(change))
    case clientproto.LocalFileDelete:
        fmt.Println("Deleted file:", changeToString(change))
    case clientproto.LocalFileUpdate:
        fmt.Println("Updated file:", changeToString(change))
    }
}

// changeToString represents a LocalFileChange as a string.
func changeToString(change *clientproto.LocalFileChange) string {
    return fmt.Sprintf("%s @ %s: (%d / %d) chunks",
        change.LocalFile.Torrent.ID,
        change.LocalFile.Path,
        len(change.LocalFile.Chunks),
        torrent.NumChunks(change.LocalFile.Torrent))
}

// processInputs gets inputs from users and acts on them.
func processInputs(c client.Client, localFiles map[torrentproto.ID]*clientproto.LocalFile, trackerNodes []torrentproto.TrackerNode) {
    fmt.Println(COMMANDS)

    var cmd, filePath, torrentPath, name string
    for {
        fmt.Scanln(&cmd, &filePath, &torrentPath, &name)
        switch cmd {
        case "CREATE":
            // Create a new torrent file.
            if filePath == "" || torrentPath == "" || name == "" {
                fmt.Println(COMMANDS)
            } else if t, err := torrent.New(filePath, name, trackerNodes); err != nil {
                fmt.Println("Could not create torrent", err)
            } else if err := torrent.Save(t, torrentPath); err != nil {
                fmt.Println("Could not write torrent", err)
            }

        case "OFFER":
            // Offer a file described by a torrent.
            if filePath == "" || torrentPath == "" {
                fmt.Println(COMMANDS)
            } else if t, err := torrent.Load(torrentPath); err != nil {
                fmt.Println("Could not read torrent file", err)
            } else if err := c.OfferFile(t, filePath); err != nil {
                fmt.Println("Could not offer data file", err)
            }

        case "DOWNLOAD":
            // Download the file described by a torrent.
            if filePath == "" || torrentPath == "" {
                fmt.Println(COMMANDS)
            } else if t, err := torrent.Load(torrentPath); err != nil {
                fmt.Println("Could not read torrent file", err)
            } else if err := c.DownloadFile(t, filePath); err != nil {
                fmt.Println("Could not download data file", err)
            }

        case "EXIT":
            // Save the client's state to a file.
            if localFilesBytes, err := json.Marshal(localFiles); err != nil {
                fmt.Println("Could not save client state", err)
            } else if err := ioutil.WriteFile(SAVE_PATH, localFilesBytes, MODE); err != nil {
                fmt.Println("Could not save client state", err)
            }

            // Quit.
            return

        default:
            // Invalid command. Print command information.
            fmt.Println(COMMANDS)
        }
    }
}

// TODO: if saving state doesn't work correctly, check how we're creating
// localFiles here, passing it into client, and then assuming that it
// will see all changed up until we serialize it in processInputs
func main() {
    // Load saved localFiles, if they exist.
    var localFiles map[torrentproto.ID]*clientproto.LocalFile
    if savedBytes, err := ioutil.ReadFile(SAVE_PATH); err != nil {
        fmt.Println("Could not find saved state", err)
    } else if err := json.Unmarshal(savedBytes, localFiles); err != nil {
        fmt.Println("Could not read saved state", err)
        localFiles = make(map[torrentproto.ID]*clientproto.LocalFile)
    }

    // Get hostports from command line.
    // First hostport is for Client, and remainder are for Trackers.
    //
    // TODO: make sure that the indices in the os.Args are correct
    if len(os.Args) < 3 {
        fmt.Println(USAGE)
        return
    }
    clientHostPort := os.Args[1]
    trackerNodes := make([]torrentproto.TrackerNode, 0)
    for _, trackerHostPort := range os.Args[2:] {
        trackerNodes = append(trackerNodes, torrentproto.TrackerNode{HostPort: trackerHostPort})
    }

    // Create an start a Client.
    lfl := & clientFileListener {}
    if c, err := client.NewClient(localFiles, lfl, clientHostPort); err != nil {
        fmt.Println("Could not start client", err)
    } else {
        // Accept commands from stdin until the user exits.
        processInputs(c, localFiles, trackerNodes)
    }
}

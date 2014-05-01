package main

import (
    "fmt"
    "math/rand"
    "os"
    "strings"
    "time"

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
        "\t<program_name> <pretty print> <client host:port> <tracker 0 host:port> ... <tracker n-1 host:port>",
        ""}, "\n")
    COMMANDS string = strings.Join([]string{
        "Commands:",
        "\tCREATE <file_path> <name>",
        "\tREGISTER <torrent_path>",
        "\tOFFER <file_path> <torrent_path>",
        "\tDOWNLOAD <file_path> <torrent_path>",
        "\tREAD <torrent_path>",
        "\tEXIT",
        ""}, "\n")
    WELCOME string = strings.Join([]string{
        "",
        "\t\tWelcome to ByteTorrent!",
        "",
        "\t%s",
        "",
        "\t\tBilly Wood, Connor Brem",
        ""}, "\n")
    TAGLINES []string = []string{
        "If you can't trust the peers, why trust the protocol?",
        "For when BitTorrent isn't sketchy enough!",
        "The security is just as questionable as the legality!",
        "'I love ByteTorrent' - William (Bill) Gates",
        "For when BitTorrent isn't big enough to hold all the bits",
        "The world's 723rd most popular P2P system"}

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
func processInputs(c client.Client, localFiles map[torrentproto.ID]*clientproto.LocalFile, trackerNodes []torrentproto.TrackerNode, prettyPrint bool) {
    var cmd string
    var args [3]string
    for {
        // Get a line of input.
        // If we're read all input (e.g. if we're reading from a temporary file
        // which another process is writing to), continue until it resumes.
        // Note that reading EOF is normal, so we don't look for this.
        if n, _ := fmt.Scanln(&cmd, &args[0], &args[1], &args[2]); n == 0 {
            continue
        }

        // Take appropriate action for 
        switch cmd {
        case "CREATE":
            // Create a new torrent file.
            filePath, name := args[0], args[1]
            torrentPath := fmt.Sprintf("%s.torrent", name)
            if filePath == "" || name == "" {
                fmt.Println(COMMANDS)
            } else if t, err := torrent.New(filePath, name, trackerNodes); err != nil {
                fmt.Println("Could not create torrent:", err)
            } else if err := torrent.Save(t, torrentPath); err != nil {
                fmt.Println("Could not write torrent:", err)
            } else {
                fmt.Println("Successfully created torrent")
            }

        case "REGISTER":
            // Register a new torrent with the tracker.
            torrentPath := args[0]
            if torrentPath == "" {
                fmt.Println(COMMANDS)
            } else if t, err := torrent.Load(torrentPath); err != nil {
                fmt.Println("Could not read torrent file:", err)
            } else if err := torrent.Register(t); err != nil {
                fmt.Println("Could not register torrent:", err)
            } else {
                fmt.Println("Successfully registered torrent")
            }

        case "OFFER":
            // Offer a file described by a torrent.
            filePath, torrentPath := args[0], args[1]
            if filePath == "" || torrentPath == "" {
                fmt.Println(COMMANDS)
            } else if t, err := torrent.Load(torrentPath); err != nil {
                fmt.Println("Could not read torrent file:", err)
            } else if err := c.OfferFile(t, filePath); err != nil {
                fmt.Println("Could not offer data file:", err)
            } else {
                fmt.Println("Successfully offered data file")
            }

        case "DOWNLOAD":
            // Download the file described by a torrent.
            filePath, torrentPath := args[0], args[1]
            if filePath == "" || torrentPath == "" {
                fmt.Println(COMMANDS)
            } else if t, err := torrent.Load(torrentPath); err != nil {
                fmt.Println("Could not read torrent file:", err)
            } else if err := c.DownloadFile(t, filePath); err != nil {
                fmt.Println("Could not download data file:", err)
            } else {
                fmt.Println("Successfully downloaded data file")
            }

        case "READ":
            // Show a human-readable representation of a torrent.
            torrentPath := args[0]
            if torrentPath == "" {
                fmt.Println(COMMANDS)
            } else if t, err := torrent.Load(torrentPath); err != nil {
                fmt.Println("Could not read torrent file:", err)
            } else {
                fmt.Println(torrent.String(t))
            }

        case "EXIT":
            // Exit the client.
            fmt.Println("Exiting")
            return

        default:
            // Invalid command. Print command information.
            if prettyPrint {
                fmt.Println(COMMANDS)
            }
        }
    }
}

func main() {
    // Don't use any saved state.
    localFiles := make(map[torrentproto.ID]*clientproto.LocalFile)

    // Get hostports from command line.
    // First hostport is for Client, and remainder are for Trackers.
    if len(os.Args) < 4 {
        fmt.Println(USAGE)
        return
    }
    prettyPrint := (os.Args[1] == "yes")
    clientHostPort := os.Args[2]
    trackerNodes := make([]torrentproto.TrackerNode, 0)
    for _, trackerHostPort := range os.Args[3:] {
        trackerNodes = append(trackerNodes, torrentproto.TrackerNode{HostPort: trackerHostPort})
    }

    // Create an start a Client.
    lfl := & clientFileListener {}
    if c, err := client.NewClient(localFiles, lfl, clientHostPort); err != nil {
        fmt.Println("Could not start client:", err)
    } else {
        // Print welcome message.
        if prettyPrint {
            r := rand.New(rand.NewSource(time.Now().UnixNano()))
            tagline := TAGLINES[r.Int() % len(TAGLINES)]
            fmt.Println(fmt.Sprintf(WELCOME, tagline))
            fmt.Println(COMMANDS)
        }

        // Accept commands from stdin until the user exits.
        processInputs(c, localFiles, trackerNodes, prettyPrint)
    }
}

package main

import (
    "encoding/json"
    "fmt"
    "strings"

    "src/torrent"
    "src/client"
)

const (
    SAVE_PATH string = "state.txt"
    USAGE string = strings.Join(string[]{
        "USAGE:",
        "CREATE <file_path> <torrent_path> <name>",
        "OFFER <file_path> <torrent_path>",
        "DOWNLOAD <file_path> <torrent_path>",
        "EXIT"
    }, "\n")
    MODE int = 644
)

// A listener which updates the view when the Client changes local files.
type clientFileListener struct {}

func (cfl *clientFileListener) OnChange(change *client.LocalFileChange) {
    switch change.Operation {
    case client.Add:
        fmt.Println("Added file:", changeToString(change))
    case client.Delete:
        fmt.Println("Deleted file:", changeToString(change))
    case client.Update:
        fmt.Println("Updated file:", changeToString(change))
    }
}

// changeToString represents a LocalFileChange as a string.
func changeToString(change *client.LocalFileChange) string {
    return fmt.Sprintf("%s @ %s: (%d / %d) chunks",
        change.LocalFile.Torrent.ID,
        change.LocalFile.Path,
        len(change.LocalFile.Chunks),
        change.LocalFile.Torrent.NumChunks())
}

// processInputs gets inputs from users and acts on them.
func processInputs(c *client.Client) {
    var cmd, filePath, torrentPath, name string
    for {
        fmt.Scanln(&cmd, &filePath, &torrentPath, &name)
        switch cmd {
        case "CREATE":
            // Create a new torrent file.
            if filePath == "" || torrentPath == "" || name == "" {
                fmt.Println(USAGE)
            } else if t, err := torrent.New(filePath, name, TRACKER_NODES); err != nil {
                fmt.Println("Could not create torrent")
            } else if torrentBytes, err := json.Marshal(t); err != nil {
                fmt.Println("Count not create torrent")
            } else if err := ioutil.WriteFile(torrentPath, torrentBytes, MODE); err != nil {
                fmt.Println("Could not write torrent")
            }

        case "OFFER":
            // Offer a file described by a torrent.
            var t torrent.Torrent
            if filePath == "" || torrentPath == "" {
                fmt.Println(USAGE)
            } else if torrentBytes, err := ioutil.ReadFile(torrentPath); err != nil {
                fmt.Println("Could not read torrent file")
            } else if err := json.Unmarshal(torrentBytes, &t); err != nil {
                fmt.Println("Could not read torrent file")
            } else if err := c.OfferFile(&t, filePath); err != nil {
                fmt.Println("Count not offer data file")
            }

        case "DOWNLOAD":
            // Download the file described by a torrent.
            if filePath == "" || torrentPath == "" {
                fmt.Println(USAGE)
            } else if torrentBytes, err := ioutil.ReadFile(torrentPath); err != nil {
                fmt.Println("Could not read torrent file")
            } else if err := json.Unmarshal(torrentBytes, &t); err != nil {
                fmt.Println("Could not read torrent file")
            } else if err := c.DownloadFile(&t, filePath); err != nil {
                fmt.Println("Count not download data file")
            }

        case "EXIT":
            // Save the client's state to a file.
            if localFilesBytes, err := json.Marshal(c.getLocalFiles()); err != nil {
                fmt.Println("Could not save client state")
            } else if err := ioutil.WriteFile(SAVE_PATH, localFilesBytes, MODE); err != nil {
                fmt.Println("Could not save client state")
            }

            // Quit.
            return

        default:
            // Invalid command. Print usage information.
            fmt.Println(USAGE)
        }
    }
}

func main() {
    // Load saved localFiles, if they exist.
    var localFiles map[string]*client.LocalFile
    if savedBytes, err := ioutil.ReadFile(SAVE_PATH); err != nil {
        fmt.Println("Could not find saved state")
    } else if err := json.Unmarshal(savedBytes, localFiles); err != nil {
        fmt.Println("Could not read saved state")
        localFiles = make(map[string]*client.LocalFile)
    }

    // Create an start a Client.
    lfl := & clientFileListener {}
    c := client.New(localFiles, lfl)

    // Accept commands from stdin until the user exits.
    processInputs(c)
}

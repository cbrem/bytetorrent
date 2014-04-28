package main

import (
    "os"

    "dummytracker"
)

var (
    USAGE string = strings.Join([]string{
        "Usage:",
        "\t<program_name> <tracker host:port>",
        ""}, "\n")
)

func main() {
    // Exit if correct args are not supplied.
    if len(os.Args) != 2 {
        fmt.Println(USAGE)
        return
    }
    hostPort := os.Args[1]

    // Start tracker on given hostport.
    dummytracker.NewTrackerServer(hostPort)

    // Block forever.
    select {}
}

package main

import (
    "fmt"
    "os"
    "strings"

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
    if _, err := dummytracker.New(hostPort); err != nil {
        fmt.Println("Failed to start dummy tracker", err)
    } else {
        fmt.Println("Started dummy tracker with hostPort =", hostPort)
    }


    // Block forever.
    select {}
}

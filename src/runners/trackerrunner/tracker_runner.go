package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"tracker"
)

var (
	USAGE string = strings.Join([]string{
		"Usage:",
		"\t<program_name> <tracker port> <tracker numNodes> <optional master hostPort>",
		""}, "\n")
)

func main() {
	// Exit if correct args are not supplied.
	if len(os.Args) != 4 && len(os.Args) != 3 {
		fmt.Println(USAGE)
		return
	}
	port := strconv.Atoi(os.Args[1])
	numNodes := strconv.Atoi(os.Args[2])
	var master string
	if len(os.Args) == 4 {
		master = os.Args[3]
	} else {
		master = ""
	}

	// Start tracker on given hostport.
	if _, err := tracker.NewTrackerServer(master, numNodes, port, nodeID); err != nil {
		fmt.Println("Failed to start tracker", err)
	} else {
		fmt.Println("Started tracker with hostPort =", hostPort)
	}

	// Block forever.
	select {}
}

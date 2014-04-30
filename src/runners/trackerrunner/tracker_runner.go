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
		"\t<program_name> <tracker port> <tracker numNodes> <tracker nodeID> <optional master hostPort>",
		""}, "\n")
)

func main() {
	// Exit if correct args are not supplied.
	if len(os.Args) != 5 && len(os.Args) != 4 {
		fmt.Println(USAGE)
		return
	}
	port, _ := strconv.Atoi(os.Args[1])
	numNodes, _ := strconv.Atoi(os.Args[2])
	nodeID, _ := strconv.Atoi(os.Args[3])
	var master string
	if len(os.Args) == 5 {
		master = os.Args[4]
	} else {
		master = ""
	}

	// Start tracker on given hostport.
	if _, err := tracker.NewTrackerServer(master, numNodes, port, nodeID); err != nil {
		fmt.Println("Failed to start tracker", err)
	} else {
		fmt.Println("Started tracker with hostPort =", port)
	}

	// Block forever.
	select {}
}

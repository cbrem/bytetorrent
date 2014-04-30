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
	if t, err := tracker.NewTrackerServer(master, numNodes, port, nodeID); err != nil {
		fmt.Println("Failed to start tracker", err)
	} else {
		fmt.Println("Started tracker with hostPort =", port)

		// Continually get a number of seconds to stall from stdin,
		// and instruct the tracker to stall for that many seconds.
		for {
			var stallSeconds int
			if n, _ := fmt.Scanln(&stallSeconds); n != 0 {
				t.DebugStall(stallSeconds)

				// Note that a stall time of 0 seconds will cause the tracker
				// to close. In this case, the runner will exit as well.
				if stallSeconds == 0 {
					return
				}
			}
		}
	}
}

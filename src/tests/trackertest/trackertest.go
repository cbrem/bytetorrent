package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"regexp"
	"rpc/trackerrpc"
	"time"
	"torrent"
	"tracker"
)

type trackerTester struct {
	srv        *rpc.Client
	myhostport string
}

type testFunc struct {
	name string
	f    func()
}

var LOGE = log.New(os.Stderr, "", log.Lshortfile|log.Lmicroseconds)

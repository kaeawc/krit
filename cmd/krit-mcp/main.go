package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/kaeawc/krit/internal/mcp"
)

var version = "dev"

func main() {
	versionFlag := flag.Bool("version", false, "Print version")
	verboseFlag := flag.Bool("verbose", false, "Enable lifecycle logging to stderr")
	flag.BoolVar(verboseFlag, "v", false, "Alias for --verbose")
	flag.Parse()

	if *versionFlag {
		fmt.Println("krit-mcp", version)
		os.Exit(0)
	}

	log.SetOutput(os.Stderr)
	if *verboseFlag {
		log.Println("krit-mcp starting...")
	}

	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	server := mcp.NewServer(reader, writer)
	server.Verbose = *verboseFlag
	server.Run()
}

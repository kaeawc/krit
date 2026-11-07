// Command oraclediff compares two krit-fir oracle JSON files per file and per expression, to check
// that an analysis mode produces the same type facts as the reference mode.
//
//	oraclediff -a whole.json -b module.json -root /path/to/repo [-detail Name.kt] [-examples 5]
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	aPath := flag.String("a", "", "reference oracle JSON")
	bPath := flag.String("b", "", "candidate oracle JSON")
	root := flag.String("root", "", "path prefix stripped from file paths before comparing")
	detail := flag.String("detail", "", "print the differing expressions of files whose path contains this text")
	examples := flag.Int("examples", 5, "example files printed per difference kind")
	flag.Parse()
	if *aPath == "" || *bPath == "" {
		fmt.Fprintln(os.Stderr, "usage: oraclediff -a ref.json -b candidate.json [-root dir]")
		os.Exit(2)
	}
	a, err := LoadFile(*aPath, *root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	b, err := LoadFile(*bPath, *root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	r := Compare(a, b, *detail)
	r.Print(os.Stdout, *examples)
}

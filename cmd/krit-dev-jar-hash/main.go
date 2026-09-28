package main

import (
	"fmt"
	"os"

	"github.com/kaeawc/krit/internal/devjar"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: krit-dev-jar-hash krit-fir|krit-types")
		os.Exit(2)
	}
	root := devjar.CheckoutRoot(os.Args[1], nil)
	if root == "" {
		fmt.Fprintln(os.Stderr, "not in a Krit source checkout")
		os.Exit(1)
	}
	hash, err := devjar.SourceHash(root, os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(hash)
}

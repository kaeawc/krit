// Command findingsdiff compares two krit JSON outputs by rule, normalized file path, line and
// message, ignoring order, and prints the differences grouped by rule. Use it to check that an
// analysis mode (for example module sharding) reports the same findings as the reference mode.
//
//	findingsdiff -a whole.json -b module.json -root /path/to/repo [-examples 5] [-list]
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	aPath := flag.String("a", "", "reference krit JSON output")
	bPath := flag.String("b", "", "candidate krit JSON output")
	root := flag.String("root", "", "path prefix stripped from finding files before comparing")
	examples := flag.Int("examples", 3, "example differences printed per rule")
	list := flag.Bool("list", false, "print every difference as a TSV line (side, rule, file, line, message)")
	aName := flag.String("a-name", "a", "label for the reference output")
	bName := flag.String("b-name", "b", "label for the candidate output")
	flag.Parse()
	if *aPath == "" || *bPath == "" {
		fmt.Fprintln(os.Stderr, "usage: findingsdiff -a ref.json -b candidate.json [-root dir]")
		os.Exit(2)
	}
	a, err := LoadFile(*aPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	b, err := LoadFile(*bPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	d := Compare(Normalize(a, *root), Normalize(b, *root))
	Report(os.Stdout, d, *aName, *bName, *examples, *list)
	if d.Total() > 0 {
		os.Exit(1)
	}
}

// Report writes the grouped difference table. OnlyA findings are in the reference only.
func Report(w io.Writer, d Diff, aName, bName string, examples int, list bool) {
	fmt.Fprintf(w, "%-48s %8s %8s\n", "rule", aName+"-only", bName+"-only")
	for _, r := range d.Rules() {
		fmt.Fprintf(w, "%-48s %8d %8d\n", r, len(d.OnlyA[r]), len(d.OnlyB[r]))
	}
	fmt.Fprintf(w, "%-48s %8d %8d\n", "TOTAL", d.CountA(), d.CountB())
	fmt.Fprintf(w, "matching: %d\n", d.Matching)
	for _, r := range d.Rules() {
		for _, side := range []struct {
			name string
			keys []Key
		}{{aName, d.OnlyA[r]}, {bName, d.OnlyB[r]}} {
			for i, k := range side.keys {
				if list {
					fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", side.name, k.Rule, k.File, k.Line, k.Message)
				} else if i < examples {
					fmt.Fprintf(w, "  %s-only %s %s:%d %s\n", side.name, k.Rule, k.File, k.Line, truncate(k.Message, 140))
				}
			}
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n]) + "..."
}

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/store"
)

// defaultStoreDir returns the default unified store root.
// Prefers .krit/store/ under the current directory, matching the
// oracle cache convention of writing into .krit/.
func defaultStoreDir() string {
	return filepath.Join(".krit", "store")
}

// runCacheSubcommand dispatches krit cache <verb> [flags].
func runCacheSubcommand(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: krit cache <stats|clear> [--store-dir DIR]")
		return 2
	}
	verb := args[0]
	rest := args[1:]

	switch verb {
	case "stats":
		return runCacheStats(rest)
	case "clear":
		return runCacheClear(rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown cache verb %q; use stats or clear\n", verb)
		return 2
	}
}

func runCacheStats(args []string) int {
	fs := flag.NewFlagSet("cache stats", flag.ContinueOnError)
	storeDirFlag := fs.String("store-dir", defaultStoreDir(), "Unified store root directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	s := store.New(*storeDirFlag)
	st, err := s.Stats()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cache stats: %v\n", err)
		return 1
	}
	fmt.Printf("Store:   %s\n", *storeDirFlag)
	fmt.Printf("Entries: %d\n", st.EntryCount)
	fmt.Printf("Size:    %s\n", formatBytes(st.TotalBytes))
	if len(st.ByKind) > 0 {
		fmt.Println()
		kinds := []store.StoreKind{store.KindIncremental, store.KindOracle, store.KindMatrix, store.KindBaseline}
		for _, k := range kinds {
			ks, ok := st.ByKind[k]
			if !ok || ks.EntryCount == 0 {
				continue
			}
			fmt.Printf("  %-14s %d entries  %s\n", store.KindLabel(k)+":", ks.EntryCount, formatBytes(ks.TotalBytes))
		}
	}
	return 0
}

func runCacheClear(args []string) int {
	fs := flag.NewFlagSet("cache clear", flag.ContinueOnError)
	storeDirFlag := fs.String("store-dir", defaultStoreDir(), "Unified store root directory")
	ruleFlag := fs.String("rule", "", "Remove only entries related to this rule name")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	s := store.New(*storeDirFlag)
	var ruleIDs []string
	if *ruleFlag != "" {
		ruleIDs = []string{*ruleFlag}
	}
	if err := s.Invalidate(ruleIDs...); err != nil {
		fmt.Fprintf(os.Stderr, "cache clear: %v\n", err)
		return 1
	}
	if *ruleFlag != "" {
		fmt.Printf("Cleared entries for rule %q from %s\n", *ruleFlag, *storeDirFlag)
	} else {
		fmt.Printf("Cleared all entries from %s\n", *storeDirFlag)
	}
	return 0
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

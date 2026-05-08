package migrate

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/migration"
)

func Run(args []string) int {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	library := fs.String("library", "", "Library name from the migration map")
	from := fs.String("from", "", "Source version")
	to := fs.String("to", "", "Target version")
	mapPath := fs.String("map", "", "Migration map YAML path")
	configPath := fs.String("config", "", "krit config path with migration map metadata")
	root := fs.String("root", "", "Repository root. Defaults to current directory.")
	format := fs.String("format", "plain", "Output format: plain or json")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *library == "" || *from == "" || *to == "" {
		fmt.Fprintln(os.Stderr, "usage: krit migrate --library LIB --from VERSION --to VERSION [--map FILE] [--format plain|json]")
		return 1
	}
	wd := *root
	if wd == "" {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
	}
	if *mapPath == "" {
		resolved, err := mapPathFromConfig(*configPath, *library)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		*mapPath = resolved
	}
	migrationMap, err := migration.LoadMap(*mapPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	report, err := migration.Analyze(migration.Options{
		Root:    wd,
		Library: *library,
		From:    *from,
		To:      *to,
		Map:     migrationMap,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	switch *format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
	case "plain":
		printPlain(report)
	default:
		fmt.Fprintf(os.Stderr, "error: unknown format %q\n", *format)
		return 1
	}
	if len(report.Suggestions) > 0 {
		return 1
	}
	return 0
}

func mapPathFromConfig(configPath, library string) (string, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return "", err
	}
	data := cfg.Data()
	raw, ok := data["migrationMaps"]
	if !ok {
		raw = data["migrationMap"]
	}
	switch v := raw.(type) {
	case string:
		return v, nil
	case map[string]interface{}:
		if path, ok := v[library].(string); ok && path != "" {
			return path, nil
		}
	case map[interface{}]interface{}:
		if path, ok := v[library].(string); ok && path != "" {
			return path, nil
		}
	}
	return "", fmt.Errorf("no migration map path configured for %q; pass --map FILE", library)
}

func printPlain(report migration.Report) {
	for _, suggestion := range report.Suggestions {
		fmt.Printf("%s:%d:%d\n", suggestion.File, suggestion.Line, suggestion.Column)
		fmt.Printf("  symbol: %s\n", suggestion.Symbol)
		fmt.Printf("  current: %s\n", suggestion.Current)
		fmt.Printf("  suggested: %s\n", suggestion.Suggested)
		fmt.Printf("  reason: %s\n", suggestion.Reason)
	}
}

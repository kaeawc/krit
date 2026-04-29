package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/kaeawc/krit/internal/arch"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

type abiHashResult struct {
	Target string `json:"target"`
	Module string `json:"module,omitempty"`
	File   string `json:"file,omitempty"`
	Hash   string `json:"hash"`
	Inputs int    `json:"inputs"`
}

// runAbiHashSubcommand implements `krit abi-hash :module` and
// `krit abi-hash path/to/File.kt`. Output is `<target> <hash>` in plain
// mode and a JSON object with `--json`.
func runAbiHashSubcommand(args []string) int {
	fs := flag.NewFlagSet("abi-hash", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonFlag := fs.Bool("json", false, "Emit JSON instead of plain text")
	socketFlag := fs.String("socket", "", "Daemon socket path (default <root>/.krit/daemon.sock)")
	noDaemonFlag := fs.Bool("no-daemon", false, "Skip daemon dispatch even if a socket is present")

	positional, rest := splitPositional(args, 1)
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	if len(positional) == 0 {
		fmt.Fprintln(os.Stderr, "usage: krit abi-hash <:module|path/to/File.kt> [--json] [--socket PATH] [--no-daemon]")
		return 1
	}
	target := positional[0]

	if !*noDaemonFlag {
		if res, ok := tryAbiHashViaDaemon(*socketFlag, target); ok {
			return emitAbiHash(res, *jsonFlag)
		}
	}

	files, err := resolveAbiHashTarget(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	sigs := arch.ExtractAbiSignatures(files)
	res := abiHashResult{Target: target, Hash: arch.HashAbiSignatures(sigs), Inputs: len(sigs)}
	if strings.HasPrefix(target, ":") {
		res.Module = target
	} else {
		res.File = target
	}
	return emitAbiHash(res, *jsonFlag)
}

// tryAbiHashViaDaemon attempts an abi-hash request against a running daemon
// at socketPath. ok=false means the caller should fall back to in-process
// execution (no socket present, or the daemon was not reachable).
func tryAbiHashViaDaemon(socketPath, target string) (abiHashResult, bool) {
	if socketPath == "" {
		root, err := os.Getwd()
		if err != nil {
			return abiHashResult{}, false
		}
		socketPath = daemon.DefaultSocketPath(root)
	}
	if !daemon.Available(socketPath) {
		return abiHashResult{}, false
	}
	var resp daemon.AbiHashResult
	if err := daemon.Call(socketPath, daemon.VerbAbiHash, daemon.AbiHashArgs{Target: target}, &resp); err != nil {
		return abiHashResult{}, false
	}
	return abiHashResult{
		Target: resp.Target,
		Module: resp.Module,
		File:   resp.File,
		Hash:   resp.Hash,
		Inputs: resp.Inputs,
	}, true
}

func emitAbiHash(res abiHashResult, asJSON bool) int {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		if err := enc.Encode(res); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Printf("%s\t%s\n", res.Target, res.Hash)
	return 0
}

func resolveAbiHashTarget(target string) ([]*scanner.File, error) {
	if strings.HasPrefix(target, ":") {
		root, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		graph, err := module.DiscoverModules(root)
		if err != nil {
			return nil, fmt.Errorf("discovering modules: %w", err)
		}
		mod, ok := graph.Modules[target]
		if !ok {
			return nil, fmt.Errorf("module %q not found", target)
		}
		return scanModuleKotlinFiles(mod), nil
	}

	info, err := os.Stat(target)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", target, err)
	}
	if info.IsDir() {
		paths, err := scanner.CollectKotlinFiles([]string{target}, nil)
		if err != nil {
			return nil, fmt.Errorf("collecting %s: %w", target, err)
		}
		files, _ := scanner.ScanFiles(paths, runtime.NumCPU())
		return files, nil
	}
	if !strings.HasSuffix(target, ".kt") {
		return nil, fmt.Errorf("expected a .kt file or directory, got %s", target)
	}
	f, err := scanner.ParseFile(target)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", target, err)
	}
	return []*scanner.File{f}, nil
}



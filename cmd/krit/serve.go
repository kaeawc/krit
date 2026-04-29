package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kaeawc/krit/internal/arch"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

// runServeSubcommand implements `krit serve`. Long-lived process that
// warms the module graph and exposes build-integration verbs over a Unix
// socket. `krit serve --stop` sends a shutdown request to an existing
// daemon at --socket.
func runServeSubcommand(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	rootFlag := fs.String("root", ".", "Project root")
	socketFlag := fs.String("socket", "", "Unix socket path (default <root>/.krit/daemon.sock)")
	stopFlag := fs.Bool("stop", false, "Stop a running daemon at --socket")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	socketPath := *socketFlag
	if socketPath == "" {
		socketPath = daemon.DefaultSocketPath(root)
	}

	if *stopFlag {
		if err := daemon.Call(socketPath, daemon.VerbShutdown, nil, nil); err != nil {
			fmt.Fprintf(os.Stderr, "krit serve --stop: %v\n", err)
			return 1
		}
		return 0
	}

	state := newDaemonState(root)
	warmStart := time.Now()
	if err := state.warm(); err != nil {
		fmt.Fprintf(os.Stderr, "krit serve: warm: %v\n", err)
		return 1
	}
	state.warmDuration = time.Since(warmStart)

	srv := daemon.NewServer(socketPath)
	registerVerbs(srv, state)

	ctx, cancel := signalContext()
	defer cancel()
	if err := srv.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "krit serve: %v\n", err)
		return 1
	}
	fmt.Printf("krit daemon: ready (%d files, %d modules, warm in %.1fs)\n",
		state.fileCount(), state.moduleCount(), state.warmDuration.Seconds())
	srv.Wait()
	return 0
}

// signalContext returns a context cancelled on SIGINT or SIGTERM.
func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

// daemonState holds the warm in-memory project state shared across verbs.
type daemonState struct {
	root         string
	warmDuration time.Duration

	mu    sync.RWMutex
	graph *module.ModuleGraph
	files int
}

func newDaemonState(root string) *daemonState { return &daemonState{root: root} }

func (s *daemonState) warm() error {
	graph, err := module.DiscoverModules(s.root)
	if err != nil {
		return fmt.Errorf("discovering modules: %w", err)
	}
	s.mu.Lock()
	s.graph = graph
	s.files = countModuleFiles(graph)
	s.mu.Unlock()
	return nil
}

func (s *daemonState) moduleGraph() *module.ModuleGraph {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.graph
}

func (s *daemonState) moduleCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.graph == nil {
		return 0
	}
	return len(s.graph.Modules)
}

func (s *daemonState) fileCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.files
}

func countModuleFiles(graph *module.ModuleGraph) int {
	if graph == nil {
		return 0
	}
	total := 0
	for _, m := range graph.Modules {
		roots := m.SourceRoots
		if len(roots) == 0 {
			roots = []string{filepath.Join(m.Dir, "src", "main", "kotlin")}
		}
		for _, r := range roots {
			_ = filepath.Walk(r, func(_ string, info os.FileInfo, err error) error {
				if err == nil && info != nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".kt") {
					total++
				}
				return nil
			})
		}
	}
	return total
}

func registerVerbs(srv *daemon.Server, state *daemonState) {
	srv.Register(daemon.VerbStatus, func(_ context.Context, _ json.RawMessage) (any, error) {
		return daemon.StatusResult{
			Ready:       true,
			Root:        state.root,
			Modules:     state.moduleCount(),
			Files:       state.fileCount(),
			WarmSeconds: state.warmDuration.Seconds(),
		}, nil
	})
	srv.Register(daemon.VerbShutdown, func(_ context.Context, _ json.RawMessage) (any, error) {
		return map[string]bool{"ok": true}, nil
	})
	srv.Register(daemon.VerbAbiHash, func(_ context.Context, raw json.RawMessage) (any, error) {
		var args daemon.AbiHashArgs
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, fmt.Errorf("decode args: %w", err)
			}
		}
		if args.Target == "" {
			return nil, errors.New("abi-hash: target is required")
		}
		files, err := resolveAbiHashTargetForDaemon(state, args.Target)
		if err != nil {
			return nil, err
		}
		sigs := arch.ExtractAbiSignatures(files)
		hash := arch.HashAbiSignatures(sigs)
		res := daemon.AbiHashResult{Target: args.Target, Hash: hash, Inputs: len(sigs)}
		if strings.HasPrefix(args.Target, ":") {
			res.Module = args.Target
		} else {
			res.File = args.Target
		}
		return res, nil
	})
}

// resolveAbiHashTargetForDaemon mirrors resolveAbiHashTarget but uses the
// daemon's cached module graph for module-style targets.
func resolveAbiHashTargetForDaemon(state *daemonState, target string) ([]*scanner.File, error) {
	if strings.HasPrefix(target, ":") {
		graph := state.moduleGraph()
		if graph == nil {
			return nil, fmt.Errorf("no module graph (root %s has no settings file)", state.root)
		}
		mod, ok := graph.Modules[target]
		if !ok {
			return nil, fmt.Errorf("module %q not found", target)
		}
		return scanModuleKotlinFiles(mod), nil
	}

	path := target
	if !filepath.IsAbs(path) {
		path = filepath.Join(state.root, path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", target, err)
	}
	if info.IsDir() {
		paths, err := scanner.CollectKotlinFiles([]string{path}, nil)
		if err != nil {
			return nil, fmt.Errorf("collecting %s: %w", target, err)
		}
		files, _ := scanner.ScanFiles(paths, runtime.NumCPU())
		return files, nil
	}
	if !strings.HasSuffix(path, ".kt") {
		return nil, fmt.Errorf("expected a .kt file or directory, got %s", target)
	}
	f, err := scanner.ParseFile(path)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", target, err)
	}
	return []*scanner.File{f}, nil
}

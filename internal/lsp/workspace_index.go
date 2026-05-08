package lsp

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/kaeawc/krit/internal/oracle"
)

func (s *Server) configureWorkspaceIndexer() {
	s.indexMu.Lock()
	if s.userSetIndexer {
		s.indexMu.Unlock()
		return
	}
	s.indexMu.Unlock()

	root := uriToPath(s.rootURI)
	if root == "" {
		return
	}
	fallback := SourceWorkspaceIndexer{}
	if !s.useOracleDaemon {
		s.indexMu.Lock()
		s.indexer = fallback
		s.indexMu.Unlock()
		return
	}
	jar := oracle.FindJar([]string{root})
	if jar == "" {
		s.indexMu.Lock()
		s.indexer = fallback
		s.indexMu.Unlock()
		return
	}
	classpath := lspClasspath(root, s.cfg, s.initClasspath)
	s.indexMu.Lock()
	s.indexer = OracleWorkspaceIndexer{
		JARPath:   jar,
		Root:      root,
		Classpath: classpath,
		Verbose:   s.Verbose,
		Fallback:  fallback,
		Ready: func(d *oracle.Daemon) {
			s.indexMu.Lock()
			s.oracleDaemon = d
			s.indexMu.Unlock()
			s.installDaemonDecompiler(d)
			if s.OracleRefresh == nil {
				s.OracleRefresh = func(uri string, _ []byte) {
					if idx := s.oracleIndex(); idx != nil {
						idx.RemoveFile(uriToPath(uri))
					}
				}
			}
		},
	}
	s.indexMu.Unlock()
}

func (s *Server) startWorkspaceIndex(params InitializeParams) {
	if !s.indexOnInitialize {
		return
	}
	root := uriToPath(params.RootURI)
	if root == "" {
		root = params.RootPath
	}
	if root == "" {
		return
	}
	if _, err := os.Stat(root); err != nil {
		s.logInfo("workspace index skipped: root unavailable: %v", err)
		return
	}

	s.indexMu.Lock()
	if s.indexCancel != nil {
		s.indexCancel()
	}
	indexer := s.indexer
	if indexer == nil {
		indexer = SourceWorkspaceIndexer{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan struct{})
	s.indexCancel = cancel
	s.indexReady = ready
	s.indexMu.Unlock()

	go func() {
		defer close(ready)
		s.reportWorkspaceIndexBegin()
		idx, err := indexer.BuildWorkspaceIndex(ctx, root, func(done, total int) {
			s.reportWorkspaceIndexProgress(done, total)
		})
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				s.log.Warn("workspace index failed", "err", err)
				s.reportWorkspaceIndexEnd("Workspace index failed")
			}
			return
		}
		if ctx.Err() != nil {
			return
		}
		s.SetOracleIndex(idx)
		s.reportWorkspaceIndexEnd("Workspace index ready")
	}()
}

func (s *Server) cancelWorkspaceIndex() {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()
	if s.indexCancel != nil {
		s.indexCancel()
		s.indexCancel = nil
	}
}

// waitForIndexShutdown cancels any in-flight workspace-index goroutine
// and blocks until it drains, or until timeout elapses. Tests use it
// to synchronize before reading shared state (e.g. the output buffer)
// that the indexer's progress notifications also write to.
//
// Production code does not need to call this — Server.handleExit
// cancels and lets the goroutine wind down asynchronously. The
// timeout silently expires; tests that care about success failure
// can check observable side effects (oracleIndex, output buffer)
// after the call.
func (s *Server) waitForIndexShutdown(timeout time.Duration) {
	s.indexMu.Lock()
	if s.indexCancel != nil {
		s.indexCancel()
		s.indexCancel = nil
	}
	ready := s.indexReady
	s.indexMu.Unlock()
	if ready == nil {
		return
	}
	select {
	case <-ready:
	case <-time.After(timeout):
	}
}

func (s *Server) releaseOracleDaemon() {
	s.indexMu.Lock()
	d := s.oracleDaemon
	s.oracleDaemon = nil
	s.indexMu.Unlock()
	if d != nil {
		_ = d.Release()
	}
}

func (s *Server) waitForOracleIndex(timeout time.Duration) *oracle.Index {
	if idx := s.oracleIndex(); idx != nil {
		return idx
	}
	s.indexMu.Lock()
	ready := s.indexReady
	s.indexMu.Unlock()
	if ready == nil {
		return nil
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ready:
		return s.oracleIndex()
	case <-timer.C:
		return nil
	}
}

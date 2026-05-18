package lsp

import (
	"context"
	"fmt"

	"github.com/kaeawc/krit/internal/oracle"
)

type OracleWorkspaceIndexer struct {
	JARPath   string
	Root      string
	Classpath []string
	Verbose   bool
	Fallback  WorkspaceIndexer
	Ready     func(*oracle.Daemon)
}

func (o OracleWorkspaceIndexer) BuildWorkspaceIndex(ctx context.Context, root string, progress WorkspaceIndexProgress) (*oracle.Index, error) {
	if o.JARPath == "" {
		if o.Fallback != nil {
			return o.Fallback.BuildWorkspaceIndex(ctx, root, progress)
		}
		return nil, fmt.Errorf("krit-types.jar not found")
	}
	if progress != nil {
		progress(0, 1)
	}
	d, err := oracle.ConnectOrStartDaemon(o.JARPath, []string{root}, o.Classpath, o.Verbose)
	if err != nil {
		if o.Fallback != nil {
			return o.Fallback.BuildWorkspaceIndex(ctx, root, progress)
		}
		return nil, err
	}
	// Ready, when set, transfers ownership of the daemon handle to the
	// caller (the LSP server stores it in oracleDaemon and releases on
	// handleShutdown). When Ready is nil, no one else will release the
	// connection — drop it on return so the TCP socket and log file
	// handle don't leak. The daemon process itself stays alive on its
	// idle timer either way.
	releaseOnReturn := o.Ready == nil
	defer func() {
		if releaseOnReturn {
			_ = d.Release()
		}
	}()
	if o.Ready != nil {
		o.Ready(d)
	}
	data, err := d.AnalyzeAll()
	if err != nil {
		if o.Fallback != nil {
			return o.Fallback.BuildWorkspaceIndex(ctx, root, progress)
		}
		return nil, err
	}
	if progress != nil {
		progress(1, 1)
	}
	return oracle.BuildIndex(data), nil
}

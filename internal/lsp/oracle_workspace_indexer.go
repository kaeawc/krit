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

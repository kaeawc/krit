package lsp

import (
	"context"
	"runtime"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/scanner"
)

type WorkspaceIndexProgress func(done, total int)

type WorkspaceIndexer interface {
	BuildWorkspaceIndex(ctx context.Context, root string, progress WorkspaceIndexProgress) (*oracle.Index, error)
}

type SourceWorkspaceIndexer struct{}

func (SourceWorkspaceIndexer) BuildWorkspaceIndex(ctx context.Context, root string, progress WorkspaceIndexProgress) (*oracle.Index, error) {
	if root == "" {
		return oracle.BuildIndex(nil), nil
	}
	kotlin, java, err := scanner.CollectKotlinAndJavaFiles(ctx, []string{root}, nil)
	if err != nil {
		return nil, err
	}
	total := len(kotlin) + len(java)
	if progress != nil {
		progress(0, total)
	}
	kotlinFiles, errs := scanner.ScanFiles(ctx, kotlin, runtime.NumCPU())
	if len(errs) > 0 {
		return nil, errs[0]
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if progress != nil {
		progress(len(kotlinFiles), total)
	}
	javaFiles, errs := scanner.ScanJavaFiles(ctx, java, runtime.NumCPU())
	if len(errs) > 0 {
		return nil, errs[0]
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if progress != nil {
		progress(len(kotlinFiles)+len(javaFiles), total)
	}
	codeIndex := scanner.BuildIndex(kotlinFiles, runtime.NumCPU(), javaFiles...)
	return oracleIndexFromSourceIndex(codeIndex), nil
}

func oracleIndexFromSourceIndex(index *scanner.CodeIndex) *oracle.Index {
	if index == nil {
		return oracle.BuildIndex(nil)
	}
	data := &oracle.Data{Version: 1, Files: map[string]*oracle.File{}}
	for _, sym := range index.Symbols {
		if sym.FQN == "" || sym.File == "" {
			continue
		}
		file := data.Files[sym.File]
		if file == nil {
			file = &oracle.File{Package: sym.Package}
			data.Files[sym.File] = file
		}
		file.Declarations = append(file.Declarations, &oracle.Class{
			FQN:        sym.FQN,
			Kind:       sym.Kind,
			Visibility: sym.Visibility,
			Line:       sym.Line,
			Column:     1,
		})
	}
	return oracle.BuildIndex(data)
}

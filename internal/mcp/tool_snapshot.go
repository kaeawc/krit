package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	snap "github.com/kaeawc/krit/internal/snapshot"
)

type snapshotArgs struct {
	Operation string `json:"operation"`
	RepoRoot  string `json:"repo_root"`

	// info
	CommitSHA string `json:"commit_sha"`

	// timeline
	Scope  string `json:"scope"`
	Target string `json:"target"`
	Metric string `json:"metric"`

	// diff
	From string `json:"from"`
	To   string `json:"to"`
}

func (s *Server) toolSnapshot(arguments json.RawMessage) ToolResult {
	var args snapshotArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	repoRoot, err := snapshotRepoRoot(args.RepoRoot)
	if err != nil {
		return errorResult(err.Error())
	}
	root := snap.SnapshotsDir(repoRoot)

	switch args.Operation {
	case "", "status":
		return snapshotStatusResult(root)
	case "info":
		return snapshotInfoResult(root, repoRoot, args.CommitSHA)
	case "timeline":
		return snapshotTimelineResult(root, args)
	case "diff":
		return snapshotDiffResult(root, repoRoot, args.From, args.To)
	default:
		return errorResult("unknown operation: " + args.Operation + "; valid: status|info|timeline|diff")
	}
}

func snapshotRepoRoot(arg string) (string, error) {
	if arg != "" {
		return arg, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return cwd, nil
}

func snapshotStatusResult(root string) ToolResult {
	manifests, err := snap.LoadManifests(root)
	if err != nil {
		return errorResult(err.Error())
	}
	return jsonResult(manifests)
}

func snapshotInfoResult(root, repoRoot, commitSHA string) ToolResult {
	if commitSHA == "" {
		return errorResult("'commit_sha' argument is required for operation=info")
	}
	sha, err := snap.ResolveCommitSHA(repoRoot, commitSHA)
	if err != nil {
		sha = commitSHA
	}
	m, err := snap.LoadManifest(root, sha)
	if err != nil {
		return errorResult(err.Error())
	}
	return jsonResult(m)
}

func snapshotTimelineResult(root string, args snapshotArgs) ToolResult {
	scope := args.Scope
	if scope == "" {
		scope = "repo"
	}
	metric := args.Metric
	if metric == "" {
		metric = "loc"
	}
	points, err := snap.Timeline(root, snap.TimelineQuery{
		Scope:  snap.TimelineScope(scope),
		Target: args.Target,
		Metric: metric,
	})
	if err != nil {
		return errorResult(err.Error())
	}
	return jsonResult(points)
}

func snapshotDiffResult(root, repoRoot, from, to string) ToolResult {
	if from == "" || to == "" {
		return errorResult("'from' and 'to' arguments are required for operation=diff")
	}
	fromSHA, err := snap.ResolveCommitSHA(repoRoot, from)
	if err != nil {
		fromSHA = from
	}
	toSHA, err := snap.ResolveCommitSHA(repoRoot, to)
	if err != nil {
		toSHA = to
	}
	d, err := snap.Diff(root, fromSHA, toSHA)
	if err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			return errorResult(fmt.Sprintf("snapshot not found (run `krit snapshot capture` first): %v", err))
		}
		return errorResult(err.Error())
	}
	return jsonResult(d)
}

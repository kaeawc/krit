package javafacts

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/kaeawc/krit/internal/perf"
)

type Options struct {
	Java       string
	Classpath  string
	HelperMain string
	Timeout    time.Duration
}

func DefaultOptions() Options {
	return Options{
		Java:       "java",
		HelperMain: "dev.krit.javafacts.Main",
		Timeout:    30 * time.Second,
	}
}

func UnavailableWarning(err error) string {
	if err == nil {
		return ""
	}
	return "Java semantic facts unavailable; continuing with conservative source analysis: " + err.Error()
}

func Invoke(ctx context.Context, helperClasspath string, files []string, opts Options, tracker perf.Tracker) (*Facts, string, error) {
	if len(files) == 0 {
		return &Facts{Version: Version}, "", nil
	}
	if helperClasspath == "" {
		return nil, UnavailableWarning(fmt.Errorf("helper classpath is empty")), nil
	}
	if opts.Java == "" {
		opts.Java = "java"
	}
	if opts.HelperMain == "" {
		opts.HelperMain = "dev.krit.javafacts.Main"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	if _, err := exec.LookPath(opts.Java); err != nil {
		return nil, UnavailableWarning(fmt.Errorf("%s not found", opts.Java)), nil
	}
	tmp, err := os.MkdirTemp("", "krit-java-facts-*")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(tmp)
	out := filepath.Join(tmp, "facts.json")
	runCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	args := []string{"-cp", helperClasspath, opts.HelperMain, "--out", out}
	if opts.Classpath != "" {
		args = append(args, "--classpath", opts.Classpath)
	}
	args = append(args, files...)
	start := time.Now()
	output, err := exec.CommandContext(runCtx, opts.Java, args...).CombinedOutput()
	if tracker != nil {
		perf.AddEntry(tracker, "javaSemanticFacts", time.Since(start))
	}
	if err != nil {
		return nil, UnavailableWarning(fmt.Errorf("helper failed: %w: %s", err, string(output))), nil
	}
	data, err := os.ReadFile(out)
	if err != nil {
		return nil, "", err
	}
	facts, err := Parse(data)
	if err != nil {
		return nil, "", err
	}
	return facts, "", nil
}

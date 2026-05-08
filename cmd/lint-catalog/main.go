package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	baseURL    = "https://android.googlesource.com/platform/tools/base/+/refs/heads/main/lint/libs/lint-checks/src/main/java/com/android/tools/lint/checks"
	dirURL     = baseURL + "?format=JSON"
	sourceURL  = "https://android.googlesource.com/platform/tools/base/+/refs/heads/main/lint/libs/lint-checks/"
	outputDir  = "android-lint-checks"
	fetchDelay = 200 * time.Millisecond
)

// GitilesEntry represents a single entry in the Gitiles directory listing.
type GitilesEntry struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// GitilesDir represents the Gitiles directory listing response.
type GitilesDir struct {
	Entries []GitilesEntry `json:"entries"`
}

// CheckInfo holds parsed metadata about a lint check source file.
type CheckInfo struct {
	File      string   `json:"file"`
	ClassName string   `json:"className"`
	Issues    []string `json:"issues"`
	Category  string   `json:"category,omitempty"`
}

// Catalog is the top-level structure written to catalog.json.
type Catalog struct {
	Source    string      `json:"source"`
	FetchedAt string      `json:"fetchedAt"`
	Checks    []CheckInfo `json:"checks"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Create output directory.
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	// Step 1: Fetch directory listing.
	fmt.Println("Fetching directory listing...")
	body, err := httpGet(dirURL)
	if err != nil {
		return fmt.Errorf("fetching directory listing: %w", err)
	}

	// Strip Gitiles XSS protection prefix )]}'\n
	cleaned := stripGitilesPrefix(body)

	var dir GitilesDir
	if err := json.Unmarshal([]byte(cleaned), &dir); err != nil {
		return fmt.Errorf("parsing directory JSON: %w", err)
	}

	// Filter to .java files only.
	var javaFiles []string
	for _, entry := range dir.Entries {
		if entry.Type == "blob" && strings.HasSuffix(entry.Name, ".java") {
			javaFiles = append(javaFiles, entry.Name)
		}
	}

	fmt.Printf("Found %d .java files\n\n", len(javaFiles))

	// Step 2: Fetch each file.
	var checks []CheckInfo
	for i, filename := range javaFiles {
		fmt.Printf("Fetching %s (%d/%d)...\n", filename, i+1, len(javaFiles))

		fileURL := baseURL + "/" + filename + "?format=TEXT"
		b64Body, err := httpGet(fileURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  WARNING: failed to fetch %s: %v\n", filename, err)
			continue
		}

		// Decode base64 content.
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64Body))
		if err != nil {
			fmt.Fprintf(os.Stderr, "  WARNING: failed to decode %s: %v\n", filename, err)
			continue
		}

		// Save file.
		outPath := filepath.Join(outputDir, filename)
		if err := os.WriteFile(outPath, decoded, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}

		// Parse metadata.
		info := parseCheck(filename, string(decoded))
		checks = append(checks, info)

		// Rate limit.
		if i < len(javaFiles)-1 {
			time.Sleep(fetchDelay)
		}
	}

	// Step 3: Write catalog.
	catalog := Catalog{
		Source:    sourceURL,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
	}

	catalogJSON, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling catalog: %w", err)
	}

	catalogPath := filepath.Join(outputDir, "catalog.json")
	if err := os.WriteFile(catalogPath, catalogJSON, 0o644); err != nil {
		return fmt.Errorf("writing catalog: %w", err)
	}

	fmt.Printf("\nDone. Saved %d checks to %s\n", len(checks), catalogPath)
	return nil
}

func httpGet(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) // #nosec G107 -- catalog fetcher: URL is supplied by the operator, not external input
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// stripGitilesPrefix removes the )]}' XSS protection prefix from Gitiles JSON responses.
func stripGitilesPrefix(s string) string {
	// The prefix is )]}'  followed by a newline.
	if idx := strings.Index(s, "\n"); idx != -1 && idx < 10 {
		prefix := s[:idx]
		if strings.Contains(prefix, ")]}'") {
			return s[idx+1:]
		}
	}
	return s
}

var (
	// Match Issue.create("ID", ...) or Issue.create(\n"ID", ...)
	issueCreateRe = regexp.MustCompile(`Issue\.create\(\s*"([^"]+)"`)
	// Match .setCategory(Category.XXX)
	categoryRe = regexp.MustCompile(`\.setCategory\(Category\.(\w+)`)
	// Match public class ClassName
	classNameRe = regexp.MustCompile(`public\s+class\s+(\w+)`)
)

func parseCheck(filename, content string) CheckInfo {
	info := CheckInfo{
		File: filename,
	}

	// Extract class name from source; fall back to filename.
	if m := classNameRe.FindStringSubmatch(content); len(m) > 1 {
		info.ClassName = m[1]
	} else {
		info.ClassName = strings.TrimSuffix(filename, ".java")
	}

	// Extract issue IDs.
	matches := issueCreateRe.FindAllStringSubmatch(content, -1)
	seen := make(map[string]bool)
	for _, m := range matches {
		id := m[1]
		if !seen[id] {
			seen[id] = true
			info.Issues = append(info.Issues, id)
		}
	}

	// Extract category (use first match).
	if m := categoryRe.FindStringSubmatch(content); len(m) > 1 {
		info.Category = m[1]
	}

	return info
}

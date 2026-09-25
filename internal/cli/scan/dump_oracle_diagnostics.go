package scan

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/kaeawc/krit/internal/oracle"
)

// --dump-oracle-diagnostics emits this compact JSON wire format:
//
//	{
//	  "source/path.kt": [{"factoryName":"USELESS_ELVIS","line":3,"col":31,"startByte":42,"endByte":44}]
//	}
//
// Every entry always includes all five fields. A startByte/endByte pair of
// 0/0 means the compiler diagnostic did not carry a usable byte range.
type oracleDiagnosticDumpEntry struct {
	FactoryName string `json:"factoryName"`
	Line        int    `json:"line"`
	Col         int    `json:"col"`
	StartByte   int    `json:"startByte"`
	EndByte     int    `json:"endByte"`
}

type RunDumpOracleDiagnosticsOpts struct {
	Backend string
	Verbose bool
	Paths   []string
}

// RunDumpOracleDiagnosticsTo invokes the selected JVM oracle once and emits
// only its retained compiler diagnostics as JSON to out. Errors go to errOut.
func RunDumpOracleDiagnosticsTo(out, errOut io.Writer, opts RunDumpOracleDiagnosticsOpts) int {
	backend, err := oracle.ParseBackend(opts.Backend)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 2
	}
	jarPath, err := oracle.EnsureBackendJar(context.Background(), backend, opts.Paths, opts.Verbose)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 2
	}
	sourceDirs := oracle.FindSourceDirs(opts.Paths)
	if len(sourceDirs) == 0 {
		fmt.Fprintln(errOut, "error: no Kotlin source directories found")
		return 2
	}
	tmp, err := os.CreateTemp("", "krit-oracle-diagnostics-*.json")
	if err != nil {
		fmt.Fprintf(errOut, "error: create oracle diagnostics temp file: %v\n", err)
		return 2
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		fmt.Fprintf(errOut, "error: close oracle diagnostics temp file: %v\n", err)
		return 2
	}
	defer os.Remove(tmpPath)
	if _, err := oracle.Invoke(jarPath, sourceDirs, tmpPath, opts.Verbose); err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		fmt.Fprintf(errOut, "error: read oracle diagnostics: %v\n", err)
		return 1
	}
	var oracleData oracle.Data
	if err := json.Unmarshal(data, &oracleData); err != nil {
		fmt.Fprintf(errOut, "error: decode oracle diagnostics: %v\n", err)
		return 1
	}
	report := make(map[string][]oracleDiagnosticDumpEntry)
	for path, file := range oracleData.Files {
		if file == nil {
			continue
		}
		entries := make([]oracleDiagnosticDumpEntry, 0, len(file.Diagnostics))
		for _, diagnostic := range file.Diagnostics {
			if diagnostic == nil {
				continue
			}
			entries = append(entries, oracleDiagnosticDumpEntry{FactoryName: diagnostic.FactoryName, Line: diagnostic.Line, Col: diagnostic.Col, StartByte: diagnostic.StartByte, EndByte: diagnostic.EndByte})
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Line != entries[j].Line {
				return entries[i].Line < entries[j].Line
			}
			if entries[i].Col != entries[j].Col {
				return entries[i].Col < entries[j].Col
			}
			return entries[i].FactoryName < entries[j].FactoryName
		})
		report[path] = entries
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(errOut, "error: encode oracle diagnostics: %v\n", err)
		return 1
	}
	if _, err := fmt.Fprintln(out, string(encoded)); err != nil {
		fmt.Fprintf(errOut, "error: write oracle diagnostics: %v\n", err)
		return 1
	}
	return 0
}

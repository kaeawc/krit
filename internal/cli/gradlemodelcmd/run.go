// Package gradlemodelcmd implements `krit gradle-model`.
package gradlemodelcmd

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/gradlemodel"
)

const usage = `usage: krit gradle-model key [--root DIR] [--write] [--list]

Prints the Gradle model inputs key: a hash of the files that can change what
kritExportModel writes (settings and build scripts, version catalogs, lockfiles,
gradle/verification-metadata.xml, gradle.properties, local.properties, the
wrapper's Gradle version, and the sources of buildSrc and of every build
nested under the root, such as an included build-logic), by root-relative
path and content. Cache .krit/gradle-model under this key to skip Gradle when
the build has not changed.

  --root DIR  build root (default: the nearest directory with a settings file)
  --write     also stamp the key and its inputs' hashes in
              DIR/.krit/gradle-model/inputs.key, so krit warns when the build no
              longer matches the model (kritExportModel removes the stamp)
  --list      print the files the key covers instead of the key
`

// Run implements `krit gradle-model`.
func Run(args []string) int { return run(args, os.Stdout, os.Stderr) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "key" {
		fmt.Fprint(stderr, usage)
		return 2
	}
	fs := flag.NewFlagSet("gradle-model key", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }
	rootFlag := fs.String("root", "", "build root")
	write := fs.Bool("write", false, "stamp the key in the model directory")
	list := fs.Bool("list", false, "print the files the key covers")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	root := *rootFlag
	if root == "" {
		root = "."
		if roots := gradlemodel.SettingsAncestors("."); len(roots) > 0 {
			root = roots[0]
		}
	}
	if *list {
		inputs, err := gradlemodel.ModelInputs(root)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		for _, input := range inputs {
			fmt.Fprintln(stdout, input)
		}
		return 0
	}
	var key string
	var err error
	if *write {
		dir := filepath.Join(root, ".krit", "gradle-model")
		if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
			fmt.Fprintf(stderr, "error: no Gradle model at %s; run kritExportModel first\n", dir)
			return 1
		}
		key, err = gradlemodel.WriteInputsStamp(dir, root)
	} else {
		key, err = gradlemodel.InputsKey(root)
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, key)
	return 0
}

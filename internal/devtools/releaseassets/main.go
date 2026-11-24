// Command releaseassets checks that a release carries the krit-fir jar that
// released CLIs download and the Bazel module source archive that Bazel
// Central Registry entries point at, each with a checksums.txt line that
// verifies it.
//
// Before publishing, check the assembled dist directory:
//
//	go run ./internal/devtools/releaseassets --tag v1.2.3 --dir dist
//
// After uploading, check the asset names the release reports, optionally
// against the full list of names that were uploaded:
//
//	gh release view v1.2.3 --json assets -q '.assets[].name' > assets.txt
//	go run ./internal/devtools/releaseassets --tag v1.2.3 --published assets.txt --expect uploaded.txt
package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	tag := flag.String("tag", "", "release tag, e.g. v1.2.3")
	dir := flag.String("dir", "", "assembled release directory to check")
	published := flag.String("published", "", "file listing the published asset names, one per line")
	expect := flag.String("expect", "", "with --published: file listing further asset names the release must have")
	flag.Parse()
	if *tag == "" || (*dir == "") == (*published == "") {
		fmt.Fprintln(os.Stderr, "usage: releaseassets --tag TAG (--dir DIR | --published FILE)")
		os.Exit(2)
	}
	var err error
	if *dir != "" {
		err = checkDir(*dir, *tag)
	} else {
		err = checkPublishedFile(*published, *expect, *tag)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "release assets: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("release assets: %s and %s ok\n", firJarName(*tag), bazelArchiveName(*tag))
}

// firJarName is the krit-fir release asset a CLI built for tag downloads.
// It must match oracle.BackendFIR's asset name; a test pins that.
func firJarName(tag string) string { return "krit-fir-" + tag + ".jar" }

// bazelArchiveName is the Bazel module source archive that
// scripts/release/bazel-source-archive.sh builds for tag and that
// .bcr/bazel/source.template.json downloads; a test pins the template.
func bazelArchiveName(tag string) string { return "krit-bazel-" + tag + ".tar.gz" }

// bazelModuleFile is MODULE.bazel's path inside the Bazel source archive; the
// template's strip_prefix is its directory.
func bazelModuleFile(tag string) string { return "krit-bazel-" + tag + "/bazel/MODULE.bazel" }

// requiredPublished are the assets, besides platform archives, that the jar
// download, the Bazel module and their verification need.
func requiredPublished(tag string) []string {
	return []string{firJarName(tag), bazelArchiveName(tag), "checksums.txt", "checksums.txt.sig"}
}

// checkDir verifies the jar in dir is a readable jar with a manifest, that
// the Bazel source archive holds the module, and that checksums.txt lists the
// SHA-256 of both.
func checkDir(dir, tag string) error {
	sums, err := os.ReadFile(filepath.Join(dir, "checksums.txt"))
	if err != nil {
		return fmt.Errorf("missing checksums.txt: %w", err)
	}
	if err := checkAsset(dir, firJarName(tag), sums, checkJar); err != nil {
		return err
	}
	return checkAsset(dir, bazelArchiveName(tag), sums, func(data []byte) error {
		return checkTarGzHas(data, bazelModuleFile(tag))
	})
}

// checkAsset reads name from dir, runs check on its content, and requires
// sums to list its SHA-256.
func checkAsset(dir, name string, sums []byte, check func([]byte) error) error {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return fmt.Errorf("missing %s: %w", name, err)
	}
	if err := check(data); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	want, err := checksumFor(sums, name)
	if err != nil {
		return err
	}
	got := sha256.Sum256(data)
	if !bytes.Equal(got[:], want) {
		return fmt.Errorf("checksums.txt lists %x for %s, file hashes to %x", want, name, got)
	}
	return nil
}

// checkTarGzHas requires a gzip-compressed tar archive with a regular file
// named want.
func checkTarGzHas(data []byte, want string) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("not a gzip archive: %w", err)
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("no %s", want)
		}
		if err != nil {
			return fmt.Errorf("not a tar archive: %w", err)
		}
		if h.Name == want && h.Typeflag == tar.TypeReg {
			return nil
		}
	}
}

// checkJar requires a zip archive with a META-INF/MANIFEST.MF entry.
func checkJar(data []byte) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("not a jar: %w", err)
	}
	for _, f := range r.File {
		if f.Name == "META-INF/MANIFEST.MF" {
			return nil
		}
	}
	return errors.New("no META-INF/MANIFEST.MF")
}

// checksumFor returns the SHA-256 listed for name in sha256sum-format content.
func checksumFor(content []byte, name string) ([]byte, error) {
	s := bufio.NewScanner(bytes.NewReader(content))
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		sum, err := hex.DecodeString(fields[0])
		if err != nil || len(sum) != sha256.Size {
			return nil, fmt.Errorf("malformed checksums.txt line for %s", name)
		}
		return sum, nil
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("checksums.txt has no line for %s", name)
}

func checkPublishedFile(path, expectPath, tag string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var expected []string
	if expectPath != "" {
		e, err := os.Open(expectPath)
		if err != nil {
			return err
		}
		defer e.Close()
		if expected, err = readNames(e); err != nil {
			return err
		}
	}
	return checkPublished(f, tag, expected)
}

// readNames reads one asset name per line, skipping blank lines.
func readNames(r io.Reader) ([]string, error) {
	var names []string
	s := bufio.NewScanner(r)
	for s.Scan() {
		if name := strings.TrimSpace(filepath.Base(s.Text())); name != "" && name != "." {
			names = append(names, name)
		}
	}
	return names, s.Err()
}

// checkPublished verifies the published asset names include every required
// asset and every name in expected.
func checkPublished(r io.Reader, tag string, expected []string) error {
	names, err := readNames(r)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, name := range names {
		have[name] = true
	}
	var missing []string
	for _, name := range append(requiredPublished(tag), expected...) {
		if have[name] {
			continue
		}
		have[name] = true // report each missing name once
		missing = append(missing, name)
	}
	if len(missing) > 0 {
		return fmt.Errorf("release %s is missing %s", tag, strings.Join(missing, ", "))
	}
	return nil
}

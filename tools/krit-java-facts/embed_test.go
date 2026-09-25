package javafactshelper

import (
	"bytes"
	"testing"
)

func TestSourceEmbedded(t *testing.T) {
	if len(Source) == 0 {
		t.Fatal("embedded Java helper source is empty")
	}
	if !bytes.Contains(Source, []byte("package dev.jasonpearson.krit.javafacts")) {
		t.Fatal("embedded Java helper source has unexpected package")
	}
}

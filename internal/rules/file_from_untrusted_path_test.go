package rules_test

import (
	"strings"
	"testing"
)

func TestFileFromUntrustedPath_PositiveNonLiteralChild(t *testing.T) {
	findings := runRuleByName(t, "FileFromUntrustedPath", `
package test

import java.io.File

class ZipExtractor {
    fun extractEntry(zipDir: File, entryName: String, data: ByteArray) {
        val out = File(zipDir, entryName)
        out.writeBytes(data)
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "canonical-path containment") {
		t.Fatalf("expected canonical path guidance, got %q", findings[0].Message)
	}
}

func TestFileFromUntrustedPath_PositiveDotDotLiteral(t *testing.T) {
	findings := runRuleByName(t, "FileFromUntrustedPath", `
package test

import java.io.File

fun downloadAsset(cacheDir: File) {
    val out = File(cacheDir, "../evil.txt")
    out.writeText("oops")
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestFileFromUntrustedPath_NegativeCanonicalGuard(t *testing.T) {
	findings := runRuleByName(t, "FileFromUntrustedPath", `
package test

import java.io.File

class ZipExtractor {
    fun extractEntry(zipDir: File, entryName: String, data: ByteArray) {
        val out = File(zipDir, entryName)
        require(out.canonicalPath.startsWith(zipDir.canonicalPath + File.separator))
        out.writeBytes(data)
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestFileFromUntrustedPath_IgnoresSafeLiteral(t *testing.T) {
	findings := runRuleByName(t, "FileFromUntrustedPath", `
package test

import java.io.File

fun unzipAsset(cacheDir: File) {
    val out = File(cacheDir, "asset.bin")
    out.writeText("ok")
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

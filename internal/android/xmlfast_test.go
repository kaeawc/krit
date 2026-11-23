package android

import (
	"context"
	"encoding/json"
	"io/fs"
	"maps"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// checkXMLFastAgainstTreeSitter fails when parseXMLFast accepts data but
// builds a different tree than the tree-sitter path. It reports whether the
// fast path accepted.
func checkXMLFastAgainstTreeSitter(t testing.TB, name string, data []byte) bool {
	t.Helper()
	fast, ok := parseXMLFast(data)
	if !ok {
		return false
	}
	want, err := parseXMLTreeSitter(context.Background(), data)
	if err != nil {
		t.Fatalf("%s: fast path accepted input tree-sitter rejects (%v): %q", name, err, truncateForLog(data))
	}
	if !reflect.DeepEqual(fast, want) {
		got, _ := json.Marshal(fast)
		exp, _ := json.Marshal(want)
		t.Fatalf("%s: fast tree differs from tree-sitter for %q\nfast: %.2000s\nwant: %.2000s", name, truncateForLog(data), got, exp)
	}
	return true
}

func truncateForLog(data []byte) string {
	if len(data) > 600 {
		return string(data[:600]) + "..."
	}
	return string(data)
}

func collectXMLFiles(t testing.TB, roots ...string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil //nolint:nilerr // unreadable entries are skipped
			}
			if d.IsDir() {
				if name := d.Name(); name == ".git" || name == "node_modules" || name == ".gradle" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".xml") {
				return nil
			}
			if data, rerr := os.ReadFile(path); rerr == nil {
				out[path] = data
			}
			return nil
		})
	}
	return out
}

var xmlFastSamples = []string{
	"<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<!-- c -->\n<resources xmlns:tools=\"t\">\n  <string name=\"a\" >x &amp; y &#65; &#x41; <b>bold</b>]> tail</string>\n  <!-- k --><![CDATA[ cd ]]>\n</resources>\n",
	"<a>\r\n <b c = 'd\"' /></a>",
	"<a x=\"'q'\" y='\"'/>",
	"<a>  <!-- c -->  text  <!-- d --> more <b/>  after\n\t</a>",
	"<a>&lt;&gt;&quot;&apos;&nbsp;&unknown;</a>",
	"<a>\n\n   \n</a>",
	"<a></a>",
	"<a/>",
	"<a\n  x=\"1\"\n  y=\"2\"\n/>",
	"<?xml version='1.0' encoding='UTF-8' standalone='yes'?><a/>",
	"<?xml version=\"1.0\"?>\n\n<!-- one -->\n<!-- two -->\n<a b=\"c\"></a>\n<!-- after -->\n",
	"<plurals name=\"p\"><item quantity=\"one\">%d item</item><item quantity=\"other\">%d items</item></plurals>",
	"<a>é ü 中文 😀 <b>ä</b></a>",
	"<a><![CDATA[<x>]]>tail</a>",
	"<a>text<![CDATA[x]]>text2</a>",
	"<a>x</a >",
	"<a b=\"x&amp;y\"/>",
	"<ns:a ns:b=\"1\" xmlns:ns=\"u\"><ns:c/></ns:a>",
	"<a>\\'escaped\\' \\n</a>",
	"<a>   leading</a>",
	"<a>\r\n  \r\n  x</a>",
	"<a>&#10;&#x20;x</a>",
	"<resources>\n    <string name=\"x\" translatable=\"false\">Hello <xliff:g id=\"n\">%1$s</xliff:g>!</string>\n</resources>",
}

// Inputs the fast path must leave to tree-sitter.
var xmlFastDeclined = []string{
	"<!DOCTYPE a><a/>",
	"<?xml-stylesheet href=\"x\"?><a/>",
	"<a><?pi x?></a>",
	"<a>&#xZZ; text</a>",
	"<a>& b</a>",
	"<a x=\"1<2\"/>",
	"<a>]]></a>",
	"<a></b>",
	"<a>",
	"<a/><b/>",
	"<a/>text",
	"\xef\xbb\xbf<a/>",
	"<a>\x01</a>",
	"<a>\xff</a>",
	"<a x=1/>",
	"<a x=\"1\"y=\"2\"/>",
	"<a><!-- a -- b --></a>",
	"<a><!-- a ---></a>",
	"<1a/>",
	"<:a/>",
	"<ä/>",
	" <?xml version=\"1.0\"?><a/>",
	"",
	// CDATA shapes tree-sitter-xml mis-scans.
	"<a><![CDATA[]]><x/><![CDATA[]]></a>",
	"<a><![CDATA[]]>text<![CDATA[x]]>tail</a>",
	"<a><![CDATA[<b>x</b>]]]></a>",
	"<a><![CDATA[]x]]></a>",
	"<resources>\n  <string name=\"empty\"><![CDATA[]]></string>\n  <string name=\"loading\">Loading...</string>\n</resources>\n",
}

func TestParseXMLFastSamples(t *testing.T) {
	for i, src := range xmlFastSamples {
		if !checkXMLFastAgainstTreeSitter(t, "sample", []byte(src)) {
			t.Errorf("sample %d: fast path declined %q", i, src)
		}
	}
	for _, src := range xmlFastDeclined {
		if _, ok := parseXMLFast([]byte(src)); ok {
			t.Errorf("fast path accepted %q", src)
		}
	}
}

// TestParseXMLFastMatchesTreeSitterOnRepoXML compares both paths on every XML
// file in the repository (fixtures, playground, test data).
func TestParseXMLFastMatchesTreeSitterOnRepoXML(t *testing.T) {
	files := collectXMLFiles(t, filepath.Join("..", ".."))
	accepted := 0
	for path, data := range files {
		if checkXMLFastAgainstTreeSitter(t, path, data) {
			accepted++
		}
	}
	t.Logf("fast path accepted %d of %d repository XML files", accepted, len(files))
	if len(files) < 100 || accepted*2 < len(files) {
		t.Fatalf("fast path accepted %d of %d repository XML files", accepted, len(files))
	}
}

// TestParseXMLFastMatchesTreeSitterOnCorpus compares both paths on the XML of
// external checkouts listed in KRIT_XML_CORPUS (path-list separated).
func TestParseXMLFastMatchesTreeSitterOnCorpus(t *testing.T) {
	dirs := filepath.SplitList(os.Getenv("KRIT_XML_CORPUS"))
	if len(dirs) == 0 {
		t.Skip("KRIT_XML_CORPUS not set")
	}
	files := collectXMLFiles(t, dirs...)
	accepted := 0
	for path, data := range files {
		if checkXMLFastAgainstTreeSitter(t, path, data) {
			accepted++
		}
	}
	t.Logf("fast path accepted %d of %d corpus XML files", accepted, len(files))
}

// TestParseXMLFastMutations mutates samples and repository XML at random and
// checks that whatever the fast path accepts matches tree-sitter.
func TestParseXMLFastMutations(t *testing.T) {
	seeds := append([]string(nil), xmlFastSamples...)
	seeds = append(seeds, xmlFastDeclined...)
	// Visit fixtures in path order: map order would change the seed list, and
	// so the inputs the fixed-seed generator produces, on every run.
	fixtures := collectXMLFiles(t, filepath.Join("..", "..", "tests", "fixtures"))
	for _, path := range slices.Sorted(maps.Keys(fixtures)) {
		if data := fixtures[path]; len(data) < 4000 {
			seeds = append(seeds, string(data))
		}
	}
	tokens := []string{"<", ">", "/", "/>", "</", "&", ";", "&amp;", "&#", "&#x", "#", "\"", "'", "=", " ", "\n", "\r", "\t",
		"<!--", "-->", "--", "<![CDATA[", "]]>", "]", "<?", "?>", "<?xml version=\"1.0\"?>", "<a>", "</a>", "<b x='1'/>",
		"x", "é", "😀", "\xef\xbb\xbf", "\x00", ":", "-", ".", "_", "1", "a"}
	rng := rand.New(rand.NewSource(7))
	iterations := 60000
	if testing.Short() {
		iterations = 5000
	}
	accepted := 0
	for i := 0; i < iterations; i++ {
		b := []byte(seeds[rng.Intn(len(seeds))])
		for n := 1 + rng.Intn(3); n > 0; n-- {
			pos := 0
			if len(b) > 0 {
				pos = rng.Intn(len(b) + 1)
			}
			switch rng.Intn(3) {
			case 0: // insert
				tok := tokens[rng.Intn(len(tokens))]
				b = append(b[:pos:pos], append([]byte(tok), b[pos:]...)...)
			case 1: // delete
				if pos < len(b) {
					end := pos + 1 + rng.Intn(4)
					if end > len(b) {
						end = len(b)
					}
					b = append(b[:pos:pos], b[end:]...)
				}
			default: // replace one byte
				if pos < len(b) {
					tok := tokens[rng.Intn(len(tokens))]
					b = append(b[:pos:pos], append([]byte(tok), b[pos+1:]...)...)
				}
			}
		}
		if checkXMLFastAgainstTreeSitter(t, "mutation", b) {
			accepted++
		}
	}
	t.Logf("fast path accepted %d of %d mutated inputs", accepted, iterations)
}

func FuzzParseXMLFast(f *testing.F) {
	for _, s := range xmlFastSamples {
		f.Add([]byte(s))
	}
	for _, s := range xmlFastDeclined {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		checkXMLFastAgainstTreeSitter(t, "fuzz", data)
	})
}

func TestParseXMLASTUsesFastPathResult(t *testing.T) {
	SetActiveXMLParseCache(nil)
	src := []byte(xmlFastSamples[0])
	got, err := ParseXMLAST(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	want, err := parseXMLTreeSitter(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("ParseXMLAST result differs from tree-sitter")
	}
	// Declined inputs still go through tree-sitter, errors included.
	if _, err := ParseXMLAST(context.Background(), []byte("no xml here")); err == nil {
		t.Fatal("expected an error for input without a root element")
	}
}

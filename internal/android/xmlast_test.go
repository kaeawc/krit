package android

import "testing"

func TestParseXMLAST_ManifestFixture(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.app">
    <application android:name=".App">
        <activity android:name=".MainActivity" android:exported="true" />
    </application>
</manifest>`)

	root, err := ParseXMLAST(data)
	if err != nil {
		t.Fatalf("ParseXMLAST failed: %v", err)
	}
	if root.Tag != "manifest" {
		t.Fatalf("root tag = %q, want manifest", root.Tag)
	}
	if root.Line != 2 {
		t.Fatalf("root line = %d, want 2", root.Line)
	}
	if root.Attr("package") != "com.example.app" {
		t.Fatalf("package attr = %q", root.Attr("package"))
	}

	app := root.ChildByTag("application")
	if app == nil {
		t.Fatal("expected application child")
	}
	if app.Line != 3 {
		t.Fatalf("application line = %d, want 3", app.Line)
	}
	if app.Attr("android:name") != ".App" {
		t.Fatalf("application android:name = %q", app.Attr("android:name"))
	}

	activity := app.ChildByTag("activity")
	if activity == nil {
		t.Fatal("expected activity child")
	}
	if activity.Line != 4 {
		t.Fatalf("activity line = %d, want 4", activity.Line)
	}
	if activity.Attr("android:exported") != "true" {
		t.Fatalf("activity android:exported = %q", activity.Attr("android:exported"))
	}
}

func TestParseXMLAST_EmptyInput(t *testing.T) {
	_, err := ParseXMLAST([]byte{})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestParseXMLAST_ChildrenByTag(t *testing.T) {
	data := []byte(`<root>
  <item name="a"/>
  <item name="b"/>
  <item name="c"/>
  <other name="x"/>
</root>`)

	root, err := ParseXMLAST(data)
	if err != nil {
		t.Fatalf("ParseXMLAST failed: %v", err)
	}

	items := root.ChildrenByTag("item")
	if len(items) != 3 {
		t.Fatalf("expected 3 item children, got %d", len(items))
	}
	for i, expected := range []string{"a", "b", "c"} {
		if items[i].Attr("name") != expected {
			t.Fatalf("item[%d] name = %q, want %q", i, items[i].Attr("name"), expected)
		}
	}

	others := root.ChildrenByTag("other")
	if len(others) != 1 {
		t.Fatalf("expected 1 other child, got %d", len(others))
	}
}

func TestParseXMLAST_MissingAttr(t *testing.T) {
	data := []byte(`<root attr="val"/>`)
	root, err := ParseXMLAST(data)
	if err != nil {
		t.Fatalf("ParseXMLAST failed: %v", err)
	}
	if got := root.Attr("nonexistent"); got != "" {
		t.Fatalf("expected empty string for missing attr, got %q", got)
	}
}

func TestParseXMLAST_ChildByTag_NotFound(t *testing.T) {
	data := []byte(`<root><child/></root>`)
	root, err := ParseXMLAST(data)
	if err != nil {
		t.Fatalf("ParseXMLAST failed: %v", err)
	}
	if root.ChildByTag("missing") != nil {
		t.Fatal("expected nil for missing child tag")
	}
}

func TestParseXMLAST_NestedElements(t *testing.T) {
	data := []byte(`<a>
  <b>
    <c>
      <d name="deep"/>
    </c>
  </b>
</a>`)

	root, err := ParseXMLAST(data)
	if err != nil {
		t.Fatalf("ParseXMLAST failed: %v", err)
	}
	if root.Tag != "a" {
		t.Fatalf("root tag = %q, want a", root.Tag)
	}
	b := root.ChildByTag("b")
	if b == nil {
		t.Fatal("expected b child")
	}
	c := b.ChildByTag("c")
	if c == nil {
		t.Fatal("expected c child")
	}
	d := c.ChildByTag("d")
	if d == nil {
		t.Fatal("expected d child")
	}
	if d.Attr("name") != "deep" {
		t.Fatalf("d name = %q, want deep", d.Attr("name"))
	}
}

func TestParseXMLAST_TextContent(t *testing.T) {
	data := []byte(`<root><message>Hello World</message></root>`)

	root, err := ParseXMLAST(data)
	if err != nil {
		t.Fatalf("ParseXMLAST failed: %v", err)
	}
	msg := root.ChildByTag("message")
	if msg == nil {
		t.Fatal("expected message child")
	}
	if msg.Text != "Hello World" {
		t.Fatalf("text = %q, want %q", msg.Text, "Hello World")
	}
}

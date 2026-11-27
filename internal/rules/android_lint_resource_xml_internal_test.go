package rules

import "testing"

// javaIntegerDecode follows java.lang.Integer.decode: an optional sign, then decimal, 0x/0X/# hex or 0 octal, within
// int range.
func TestJavaIntegerDecode(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"10", 10, true},
		{"-10", -10, true},
		{"+7", 7, true},
		{"0x1F", 31, true},
		{"#FF", 255, true},
		{"010", 8, true},
		{"-2147483648", -2147483648, true},
		{"2147483648", 0, false},
		{"08", 0, false},
		{"0x-1", 0, false},
		{"", 0, false},
		{"abc", 0, false},
	}
	for _, c := range cases {
		got, ok := javaIntegerDecode(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("javaIntegerDecode(%q) = %d, %v; want %d, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

// parseResourceURL reads @type/name, @+id/name, @pkg:type/name and ?attr/name.
func TestParseResourceURL(t *testing.T) {
	cases := map[string]resourceURL{
		"@id/home":           {typ: "id", name: "home"},
		"@+id/home":          {typ: "id", name: "home"},
		"@android:id/list":   {typ: "id", name: "list"},
		"?attr/colorPrimary": {typ: "attr", name: "colorPrimary"},
		"@navigation/graph":  {typ: "navigation", name: "graph"},
	}
	for in, want := range cases {
		got, ok := parseResourceURL(in)
		if !ok || got != want {
			t.Errorf("parseResourceURL(%q) = %+v, %v; want %+v", in, got, ok, want)
		}
	}
	for _, in := range []string{"home", "@id", "@id/", "@/home", ""} {
		if got, ok := parseResourceURL(in); ok {
			t.Errorf("parseResourceURL(%q) = %+v, want no match", in, got)
		}
	}
}

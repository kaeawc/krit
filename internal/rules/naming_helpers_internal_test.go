package rules

import "testing"

func TestContainsIdentifierStart(t *testing.T) {
	cases := []struct {
		text   string
		prefix string
		want   bool
	}{
		{"", "Java", false},
		{"Java", "Java", true},
		{"JavaSerializer.from(input)", "Java", true},
		{"foo.JavaSerializer.from(input)", "Java", true},
		{"parseJavadoc(content)", "Java", false},
		{"registry.getJavadocFor(id)", "Java", false},
		{"base64.decodeRaJava(raw)", "Java", false},
		{"TemplateJavaxFooHelper.process(html)", "Javax", false},
		{"return JavaxHelper.run()", "Javax", true},
		{"Xjava.Bar()", "java", false},
		{".java", "java", true},
	}
	for _, c := range cases {
		got := containsIdentifierStart(c.text, c.prefix)
		if got != c.want {
			t.Errorf("containsIdentifierStart(%q, %q) = %v, want %v", c.text, c.prefix, got, c.want)
		}
	}
}

func TestContainsIdentifierStart_EmptyPrefix(t *testing.T) {
	if containsIdentifierStart("anything", "") {
		t.Fatal("empty prefix must not match")
	}
}

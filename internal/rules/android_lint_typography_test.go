package rules_test

import (
	"reflect"
	"testing"
)

const typographySample = `<resources>
    <string name="a1">Loading...</string>
    <string name="a2">Wait....</string>
    <string name="b1">Pages 10-20</string>
    <string name="b2">Pages 10 - 20</string>
    <string name="b3">It is 5 -3 degrees</string>
    <string name="b4">Wait -- what</string>
    <string name="b5">Wait--what</string>
    <string name="b6">Wait ---- what</string>
    <string name="ga_trackingId">UA-1234-5</string>
    <string name="c1">Don't stop</string>
    <string name="c2">'quoted' text</string>
    <string name="c3">He said 'hello' to me</string>
    <string name="c4">He said "hello" to me</string>
    <string name="c5">Use ` + "`" + `grave' quotes</string>
    <string name="c6">Use ` + "``" + `double'' quotes</string>
    <string name="c7">Can't won't</string>
    <string name="c8">  Don't indent</string>
    <string name="c9">
        Multi line
        don't stop
    </string>
    <string name="c10">'hello'</string>
    <string name="c11">I'm 'fine'</string>
    <string name="c12">rock 'n' roll</string>
    <string name="c13">"quoted"</string>
    <string name="c14">x "y</string>
    <string name="d1">Add 1/2 cup</string>
    <string name="d2">1/4 and 3/4 and 1/3 and 2/3</string>
    <string name="d3">10/2</string>
    <string name="d4">Item 3 / 4</string>
    <string name="e1">Copyright (c) 2024</string>
    <string name="e2">Copyright (C) 2024</string>
    <string name="f1" translatable="false">Loading...</string>
    <string name="google_app_id">1:23-45...</string>
    <string-array name="arr">
        <item>Loading...</item>
        <item>Don't</item>
        <item><![CDATA[Wait... don't]]></item>
    </string-array>
    <plurals name="pl">
        <item quantity="one">1 item...</item>
        <item quantity="other">%d items...</item>
    </plurals>
    <string name="g1"><![CDATA[Loading...]]></string>
    <string name="g2"><![CDATA[<b>Loading...</b>]]></string>
    <string name="g4">Rock &amp; roll...</string>
    <string name="g5">A&#8230;B and ...</string>
    <string name="h1">Use \'escaped\' quotes</string>
    <string name="h2">it\'s</string>
</resources>`

// Expected positions come from running Android Lint 32.4.1 on the same files.
func TestTypographyLintParity(t *testing.T) {
	files := map[string]string{
		"values/strings.xml": typographySample,
		"values-ar/strings.xml": `<resources>
    <string name="b1">الصفحات 10-20</string>
    <string name="b4">انتظر -- ماذا</string>
</resources>`,
	}
	want := map[string][]string{
		"TypographyDashes": {
			"values-ar/strings.xml:2:23", "values-ar/strings.xml:3:23", "values/strings.xml:4:23", "values/strings.xml:5:23",
			"values/strings.xml:7:23", "values/strings.xml:8:23",
		},
		"TypographyEllipsis": {
			"values/strings.xml:2:23", "values/strings.xml:37:15", "values/strings.xml:39:24", "values/strings.xml:42:30",
			"values/strings.xml:43:32", "values/strings.xml:45:32", "values/strings.xml:47:23", "values/strings.xml:48:23",
		},
		"TypographyFractions": {"values/strings.xml:28:23", "values/strings.xml:29:23", "values/strings.xml:31:23"},
		"TypographyOther":     {"values/strings.xml:32:23", "values/strings.xml:33:23"},
		"TypographyQuotes": {
			"values/strings.xml:11:26", "values/strings.xml:12:23", "values/strings.xml:13:23", "values/strings.xml:14:23",
			"values/strings.xml:15:33", "values/strings.xml:16:23", "values/strings.xml:17:23", "values/strings.xml:18:28",
			"values/strings.xml:21:12", "values/strings.xml:24:24", "values/strings.xml:25:24", "values/strings.xml:38:18",
			"values/strings.xml:39:35", "values/strings.xml:49:23", "values/strings.xml:50:26",
		},
	}
	for rule, positions := range want {
		t.Run(rule, func(t *testing.T) {
			got := resourcePositions(t, rule, files)
			if !reflect.DeepEqual(got, positions) {
				t.Fatalf("positions:\n got  %v\n want %v", got, positions)
			}
		})
	}
}

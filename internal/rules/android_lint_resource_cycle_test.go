package rules

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// issueLines renders the findings of one issue as "file | line:col | message".
func issueLines(root, issue string) []string {
	var out []string
	for _, f := range analyzeTranslations([]string{root}) {
		if f.issue != issue {
			continue
		}
		rel, _ := filepath.Rel(root, f.file)
		out = append(out, strings.Join([]string{filepath.ToSlash(rel), strconv.Itoa(f.line) + ":" + strconv.Itoa(f.col), f.msg}, " | "))
	}
	sort.Strings(out)
	return out
}

func cycleLines(root string) []string { return issueLines(root, lintResourceCycle) }

// The expectations come from running Android Lint 32.4.1 on the same files.
func TestResourceCycleValues(t *testing.T) {
	root := writeTranslationFiles(t, map[string]string{
		"values/v.xml": `<resources>
    <string name="a">@string/b</string>
    <string name="b">@string/a</string>
    <string name="self">@string/self</string>
    <string name="ok">@string/app_name</string>
    <string name="mixed">@color/mixed</string>
    <color name="c1">@color/c2</color>
    <color name="c2">@color/c3</color>
    <color name="c3">@color/c1</color>
    <dimen name="d1">@dimen/d1</dimen>
    <style name="S1" parent="S2"/>
    <style name="S2" parent="S3"/>
    <style name="S3" parent="S1"/>
    <style name="Self" parent="Self"/>
    <style name="P.Q" parent="P.Q.R"/>
    <style name="P.Q.R"/>
    <style name="Dot.Cycle"/>
    <style name="Dot" parent="Dot.Cycle"/>
    <style name="Fine.Child"/>
    <style name="Parentless" parent=""/>
    <style name="Framework" parent="@android:style/Theme"/>
    <item name="it1" type="dimen">@dimen/it1</item>
    <item name="it2" type="string">@string/it3</item>
    <item name="it3" type="string">@string/it2</item>
</resources>
`,
		"values-night/v.xml": `<resources>
    <string name="a">@string/b</string>
</resources>
`,
	})
	wantLines(t, cycleLines(root),
		"values/v.xml | 2:22 | String Resource definition cycle: a => b => a",
		"values/v.xml | 4:25 | String `self` should not reference itself",
		"values/v.xml | 7:22 | Color Resource definition cycle: c1 => c2 => c3 => c1",
		"values/v.xml | 10:22 | Dimension `d1` should not reference itself",
		"values/v.xml | 11:22 | Style Resource definition cycle: S1 => S2 => S3 => S1",
		"values/v.xml | 14:24 | Style `Self` should not extend itself",
		"values/v.xml | 15:23 | Style Resource definition cycle: P.Q => P.Q.R => P.Q",
		"values/v.xml | 18:23 | Style Resource definition cycle: Dot => Dot.Cycle => Dot",
		"values/v.xml | 23:36 | String Resource definition cycle: it2 => it3 => it2",
	)
}

func TestResourceCycleFileResources(t *testing.T) {
	root := writeTranslationFiles(t, map[string]string{
		"drawable/d1.xml": `<selector xmlns:android="http://schemas.android.com/apk/res/android">
    <item android:drawable="@drawable/d2"/>
</selector>
`,
		"drawable/d2.xml": `<selector xmlns:android="http://schemas.android.com/apk/res/android">
    <item android:drawable="@drawable/d1"/>
</selector>
`,
		"drawable/ic.xml": `<layer-list xmlns:android="http://schemas.android.com/apk/res/android">
    <item android:drawable="@drawable/ic"/>
    <item android:drawable="@drawable/other" tools:ignore="ResourceCycle" xmlns:tools="http://schemas.android.com/tools"/>
</layer-list>
`,
		"drawable/ic2.xml": `<bitmap xmlns:android="http://schemas.android.com/apk/res/android" android:src="@drawable/ic3"/>
`,
		"drawable/ic3.xml": `<inset xmlns:android="http://schemas.android.com/apk/res/android" android:drawable="@drawable/ic2"/>
`,
		"drawable/other.xml": `<shape xmlns:android="http://schemas.android.com/apk/res/android"/>
`,
		"layout/b6.xml": `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"/>
`,
		"layout-land/b6.xml": `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android">
    <include layout="@layout/b6"/>
</LinearLayout>
`,
		"color/sel.xml": `<selector xmlns:android="http://schemas.android.com/apk/res/android">
    <item android:color="@color/sel"/>
</selector>
`,
		"layout/tools.xml": `<FrameLayout xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools">
    <fragment tools:layout="@layout/tools"/>
</FrameLayout>
`,
	})
	wantLines(t, cycleLines(root),
		"drawable/d1.xml | 2:5 | Drawable Resource definition cycle: d1 => d2 => d1",
		"drawable/ic.xml | 2:5 | Drawable `ic` should not reference itself",
		"drawable/ic.xml | 2:11 | Drawable `ic` should not reference itself",
		"drawable/ic2.xml | 1:68 | Drawable Resource definition cycle: ic2 => ic3 => ic2",
		"layout-land/b6.xml | 2:14 | Layout `b6` should not include itself",
		"color/sel.xml | 2:5 | Color `sel` should not reference itself",
		"color/sel.xml | 2:11 | Color `sel` should not reference itself",
	)
}

func TestResourceCycleStyleParentForms(t *testing.T) {
	root := writeTranslationFiles(t, map[string]string{
		"values/v.xml": `<resources>
    <style name="P.Q" parent="@style/P.Q.R"/>
    <style name="Bad" parent="@string/foo"/>
    <style name="D1" parent="@style/D1"/>
    <style name="Other" parent="@style/Elsewhere"/>
</resources>
`,
	})
	wantLines(t, cycleLines(root),
		"values/v.xml | 2:23 | Potential cycle: `P.Q` is the implied parent of `P.Q.R` and this defines the opposite",
		"values/v.xml | 3:23 | Invalid parent reference: expected a @style",
		"values/v.xml | 4:22 | Style `D1` should not extend itself",
	)
}

func TestResourceCycleEntryCycleReportsAtLastName(t *testing.T) {
	root := writeTranslationFiles(t, map[string]string{
		"values/v.xml": `<resources>
    <string name="x">@string/a</string>
    <string name="a">@string/b</string>
    <string name="b">@string/a</string>
</resources>
`,
	})
	// The path x => a => b => a enters the loop from outside; Lint reports where `a` is defined.
	wantLines(t, cycleLines(root), "values/v.xml | 3:22 | String Resource definition cycle: a => b => a")
}

func TestResourceCycleRepeatedAliasChainIsLinear(t *testing.T) {
	// The same 40-level alias chain in two configurations doubles every edge; a search that revisits names would
	// take 2^40 steps.
	var chain strings.Builder
	chain.WriteString("<resources>\n")
	for i := range 40 {
		fmt.Fprintf(&chain, "    <string name=\"s%02d\">@string/s%02d</string>\n", i, i+1)
	}
	chain.WriteString("    <string name=\"a\">@string/b</string>\n    <string name=\"b\">@string/a</string>\n</resources>\n")
	root := writeTranslationFiles(t, map[string]string{"values/v.xml": chain.String(), "values-night/v.xml": chain.String()})
	done := make(chan []string, 1)
	go func() { done <- cycleLines(root) }()
	select {
	case got := <-done:
		wantLines(t, got, "values/v.xml | 42:22 | String Resource definition cycle: a => b => a")
	case <-time.After(30 * time.Second):
		t.Fatal("cycle search did not finish")
	}
}

func TestReferenceTypeTagFindings(t *testing.T) {
	root := writeTranslationFiles(t, map[string]string{
		"values/v.xml": `<resources xmlns:tools="http://schemas.android.com/tools">
    <string name="a1">@color/x</string>
    <string name="a2">@string/x</string>
    <string name="a3">@android:color/x</string>
    <string name="a4">@android:string/x</string>
    <string name="a5"> @color/x</string>
    <string name="a7">text @color/x</string>
    <string name="a8">@+id/x</string>
    <string name="a11">?attr/x</string>
    <string name="a12">@COLOR/x</string>
    <string name="a14">@color/</string>
    <color name="b1">@string/x</color>
    <color name="b3">@color/x</color>
    <dimen name="c1">@integer/x</dimen>
    <drawable name="f1">@color/x</drawable>
    <drawable name="f2">@string/x</drawable>
    <drawable name="f3">@mipmap/x</drawable>
    <attr name="at1" format="string">@string/x</attr>
    <string name="a17" tools:ignore="ReferenceType">@color/x</string>
    <plurals name="p1"><item quantity="one">@color/x</item></plurals>
</resources>
`,
	})
	var got []string
	for _, line := range issueLines(root, lintReferenceType) {
		parts := strings.SplitN(line, " | ", 3)
		got = append(got, parts[1]+" "+parts[2])
	}
	sort.Strings(got)
	const pre = "Unexpected resource reference type; expected value of type "
	wantLines(t, got,
		"2:23 "+pre+"`@string/`", "4:23 "+pre+"`@string/`", "6:24 "+pre+"`@string/`", "8:23 "+pre+"`@string/`",
		"12:22 "+pre+"`@color/`", "14:22 "+pre+"`@dimen/`", "16:25 "+pre+"`@drawable/`", "18:38 "+pre+"`@attr/`",
	)
}

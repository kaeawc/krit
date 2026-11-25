package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// appCompatDeclaredProfile is a project that declares appcompat with the given minSdk.
func appCompatDeclaredProfile(minSdk int) librarymodel.ProjectProfile {
	return librarymodel.ProjectProfile{
		HasGradle: true, DependencyExtractionComplete: true, MinSdkVersion: minSdk,
		Dependencies: []librarymodel.Dependency{{Group: "androidx.appcompat", Name: "appcompat", Version: "1.8.0"}},
	}
}

// appCompatSourceFindings runs one rule over a single source file (under relPath in a temp directory) with the
// profile's library facts and the source resolver, returning sorted "line:col message" entries.
func appCompatSourceFindings(t *testing.T, ruleID, relPath, src string, profile librarymodel.ProjectProfile) []string {
	t.Helper()
	path := filepath.Join(t.TempDir(), relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	parse := scanner.ParseFile
	if strings.HasSuffix(path, ".java") {
		parse = scanner.ParseJavaFile
	}
	file, err := parse(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	rule := buildRuleIndex()[ruleID]
	if rule == nil {
		t.Fatalf("%s not registered", ruleID)
	}
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	dispatcher := rules.NewDispatcher([]*api.Rule{rule}, resolver)
	dispatcher.SetLibraryFacts(librarymodel.FactsForProfile(profile))
	cols := dispatcher.Run(file)
	var out []string
	for _, f := range cols.Findings() {
		out = append(out, fmt.Sprintf("%d:%d %s", f.Line, f.Col, f.Message))
	}
	sort.Strings(out)
	return out
}

const appCompatCallsSample = `package com.example

import android.content.Context
import android.content.res.ColorStateList
import android.content.res.Resources
import android.graphics.PorterDuff
import android.os.Bundle
import android.view.View
import android.widget.Button
import android.widget.Switch
import android.widget.TextView
import android.widget.Toolbar
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat

class Main : AppCompatActivity() {
    private lateinit var sw: Switch
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setActionBar(Toolbar(this))
        this.setActionBar(null)
        val d = getDrawable(R.drawable.x)
        val d2 = resources.getDrawable(R.drawable.x)
        val d3 = resources.getDrawable(R.drawable.x, theme)
        val c = resources.getColorStateList(R.color.c)
        val c2 = getColorStateList(R.color.c)
        val c3 = resources.getColorStateList(R.color.c, theme)
        val ok = ContextCompat.getDrawable(this, R.drawable.x)
        val tv = TextView(this)
        tv.setCompoundDrawableTintList(c)
        tv.compoundDrawableTintList = c
        tv.setCompoundDrawableTintMode(PorterDuff.Mode.SRC_IN)
        val b: Button = Button(this)
        b.setCompoundDrawableTintList(null)
        val s = Switch(this)
        val s2 = findViewById<Switch>(1)
        tv.apply { setCompoundDrawableTintList(c) }
        with(b) { getDrawable(1) }
    }
    fun useView(v: View, ctx: Context, r: Resources, sw2: Switch) {
        v.context.getDrawable(1)
        ctx.getDrawable(1)
        r.getDrawable(1)
        v.resources.getDrawable(1)
    }
}

class MySwitch(context: Context) : Switch(context)

class MyText(context: Context) : TextView(context) {
    fun t() { setCompoundDrawableTintList(null) }
}
`

// Expected positions and messages come from running the lint.jar of androidx.appcompat:appcompat 1.8.0 through the
// Android Lint CLI on the same file, with minSdk 21 and the AndroidX classes on the classpath.
func TestAppCompatCallsLintParity(t *testing.T) {
	want := map[string][]string{
		"UseSupportActionBar": {
			"20:9 Use `AppCompatActivity.setSupportActionBar`",
			"21:9 Use `AppCompatActivity.setSupportActionBar`",
		},
		"UseCompatLoadingForDrawables": {
			"22:17 Use `AppCompatResources.getDrawable()`",
			"23:18 Use `ResourcesCompat.getDrawable()`",
			"24:18 Use `ResourcesCompat.getDrawable()`",
			// Lint also reports 38:19, `with(b) { getDrawable(1) }`: the Button has no getDrawable(int), so Kotlin
			// resolves the call to the enclosing activity. Without member tables krit cannot prove that, so it
			// abstains inside a scope function whose subject is not a Context (a pinned false negative).
			"41:9 Use `AppCompatResources.getDrawable()`",
			"42:9 Use `AppCompatResources.getDrawable()`",
			"43:9 Use `ResourcesCompat.getDrawable()`",
			"44:9 Use `ResourcesCompat.getDrawable()`",
		},
		"UseCompatLoadingForColorStateLists": {
			"25:17 Use `AppCompatResources.getColorStateList()`",
		},
		"UseCompatTextViewDrawableApis": {
			"30:9 Use `TextViewCompat.setCompoundDrawableTintList()`",
			"31:9 Use `TextViewCompat.setCompoundDrawableTintList()`",
			"32:9 Use `TextViewCompat.setCompoundDrawableTintMode()`",
			"34:9 Use `TextViewCompat.setCompoundDrawableTintList()`",
			"37:20 Use `TextViewCompat.setCompoundDrawableTintList()`",
			"51:15 Use `TextViewCompat.setCompoundDrawableTintList()`",
		},
		"UseSwitchCompatOrMaterialCode": {
			"17:5 Use `SwitchCompat` from AppCompat or `MaterialSwitch` from Material library",
			"35:9 Use `SwitchCompat` from AppCompat or `MaterialSwitch` from Material library",
			"36:9 Use `SwitchCompat` from AppCompat or `MaterialSwitch` from Material library",
			"40:54 Use `SwitchCompat` from AppCompat or `MaterialSwitch` from Material library",
			"48:36 Use `SwitchCompat` from AppCompat or `MaterialSwitch` from Material library",
		},
	}
	for rule, expected := range want {
		t.Run(rule, func(t *testing.T) {
			got := appCompatSourceFindings(t, rule, "src/main/java/com/example/K.kt", appCompatCallsSample, appCompatDeclaredProfile(21))
			sort.Strings(expected)
			if !reflect.DeepEqual(got, expected) {
				t.Fatalf("findings:\n got  %q\n want %q", got, expected)
			}
		})
	}
}

const appCompatCallsJavaSample = `package com.example;

import android.content.Context;
import android.widget.Switch;
import android.widget.TextView;
import androidx.appcompat.app.AppCompatActivity;

public class J extends AppCompatActivity {
    private Switch field;
    void go(Context ctx, TextView tv) {
        setActionBar(null);
        getDrawable(1);
        getResources().getDrawable(1);
        getResources().getColorStateList(2);
        ctx.getDrawable(1);
        tv.setCompoundDrawableTintList(null);
        Switch local = new Switch(this);
    }
}
`

// Expected positions and messages come from the Lint CLI run described above, on the same Java file.
func TestAppCompatCallsJavaLintParity(t *testing.T) {
	want := map[string][]string{
		"UseSupportActionBar":                {"11:9 Use `AppCompatActivity.setSupportActionBar`"},
		"UseCompatLoadingForDrawables":       {"12:9 Use `AppCompatResources.getDrawable()`", "13:9 Use `ResourcesCompat.getDrawable()`", "15:9 Use `AppCompatResources.getDrawable()`"},
		"UseCompatLoadingForColorStateLists": {"14:9 Use `AppCompatResources.getColorStateList()`"},
		"UseCompatTextViewDrawableApis":      {"16:9 Use `TextViewCompat.setCompoundDrawableTintList()`"},
		"UseSwitchCompatOrMaterialCode": {
			"17:9 Use `SwitchCompat` from AppCompat or `MaterialSwitch` from Material library",
			"9:5 Use `SwitchCompat` from AppCompat or `MaterialSwitch` from Material library",
		},
	}
	for rule, expected := range want {
		t.Run(rule, func(t *testing.T) {
			got := appCompatSourceFindings(t, rule, "src/main/java/com/example/J.java", appCompatCallsJavaSample, appCompatDeclaredProfile(21))
			sort.Strings(expected)
			if !reflect.DeepEqual(got, expected) {
				t.Fatalf("findings:\n got  %q\n want %q", got, expected)
			}
		})
	}
}

// The checks ship in the appcompat AAR: a project that does not declare appcompat (or Material) gets none of them,
// and Lint skips test sources.
func TestAppCompatCallChecksNeedAppCompatAndMainSources(t *testing.T) {
	noAppCompat := librarymodel.ProjectProfile{HasGradle: true, DependencyExtractionComplete: true, MinSdkVersion: 21}
	for _, rule := range []string{"UseSupportActionBar", "UseCompatLoadingForDrawables", "UseCompatLoadingForColorStateLists",
		"UseCompatTextViewDrawableApis", "UseSwitchCompatOrMaterialCode"} {
		if got := appCompatSourceFindings(t, rule, "src/main/java/com/example/K.kt", appCompatCallsSample, noAppCompat); len(got) != 0 {
			t.Errorf("%s without appcompat: got %q, want none", rule, got)
		}
		if got := appCompatSourceFindings(t, rule, "src/test/java/com/example/KTest.kt", appCompatCallsSample, appCompatDeclaredProfile(21)); len(got) != 0 {
			t.Errorf("%s in test sources: got %q, want none", rule, got)
		}
	}
}

// From minSdk 24 Lint suggests ContextCompat.getColorStateList; below it (and when minSdk is unknown, which Lint
// treats as API 1) it suggests AppCompatResources.
func TestUseCompatLoadingForColorStateListsMessageFollowsMinSdk(t *testing.T) {
	for minSdk, msg := range map[int]string{
		0:  "25:17 Use `AppCompatResources.getColorStateList()`",
		23: "25:17 Use `AppCompatResources.getColorStateList()`",
		24: "25:17 Use `ContextCompat.getColorStateList()`",
	} {
		got := appCompatSourceFindings(t, "UseCompatLoadingForColorStateLists", "src/main/java/com/example/K.kt", appCompatCallsSample, appCompatDeclaredProfile(minSdk))
		if !reflect.DeepEqual(got, []string{msg}) {
			t.Errorf("minSdk %d: got %q, want %q", minSdk, got, msg)
		}
	}
}

// Inside a scope function whose subject type is unknown, the implicit receiver cannot be proven, so the TextView
// setter is not reported; Lint resolves `holder.label` and reports 13:26 (a pinned false negative). A subject with a
// known non-TextView type is not reported, by Lint either.
func TestUseCompatTextViewDrawableApisScopeSubjects(t *testing.T) {
	src := `package com.example

import android.content.res.ColorStateList
import android.widget.TextView

class Holder(val label: TextView)

class Chips {
    fun setCompoundDrawableTintList(colors: ColorStateList?) {}
}

fun bind(holder: Holder, colors: ColorStateList, chips: Chips, label: TextView) {
    holder.label.apply { setCompoundDrawableTintList(colors) }
    chips.apply { setCompoundDrawableTintList(colors) }
    with(label) { setCompoundDrawableTintList(colors) }
}
`
	got := appCompatSourceFindings(t, "UseCompatTextViewDrawableApis", "src/main/java/com/example/Bind.kt", src, appCompatDeclaredProfile(21))
	want := []string{"15:19 Use `TextViewCompat.setCompoundDrawableTintList()`"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

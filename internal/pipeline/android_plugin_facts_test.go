package pipeline

import (
	"reflect"
	"testing"
)

// modulePlugins finds Android plugin ids, id("...") and alias(...) declarations; an id call counts only at a word
// boundary, so a longer name ending in "id" is no plugin.
func TestModulePluginsFindsDeclarations(t *testing.T) {
	script := "plugins {\n    id(\"org.jetbrains.kotlin.android\")\n    alias(libs.plugins.android.library)\n    id 'kotlin-kapt'\n}\n" +
		"apply plugin: 'com.android.application'\nandroid { valid(\"not a plugin\"); android_id(\"neither\") }\n"
	want := []string{"com.android.application", `id("org.jetbrains.kotlin.android"`, "id 'kotlin-kapt'", "alias(libs.plugins.android.library)"}
	if got := modulePlugins(script); !reflect.DeepEqual(got, want) {
		t.Fatalf("modulePlugins = %q, want %q", got, want)
	}
}

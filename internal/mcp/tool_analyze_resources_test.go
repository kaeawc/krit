package mcp

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func analyzeAndroidResources(t *testing.T, root string) string {
	t.Helper()
	args, _ := json.Marshal(analyzeArgs{Mode: "android", ProjectPath: root, Scope: "resources"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "analyze", Arguments: args}),
	})
	data, _ := json.Marshal(responses[0].Result)
	return string(data)
}

// OnClick reads the project's sources to check a layout's onClick handlers;
// the android mode used to hand it none, so it never reported (#1860).
func TestAnalyzeAndroidResourcesChecksOnClickHandlers(t *testing.T) {
	const activity = `package p

import android.app.Activity
import android.os.Bundle
import android.view.View

class MainActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)
    }
%s}
`
	for _, tc := range []struct {
		name, handler string
		relative      bool
		want          bool
	}{
		{"handler missing", "", false, true},
		{"handler declared", "\n    fun onSubmitClicked(v: View) {}\n", false, false},
		// The res dirs are absolute; the sources once stayed relative, so no
		// Activity matched the res dir's module (#1908).
		{"handler missing, relative projectPath", "", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeProjectFile(t, root, "src/main/AndroidManifest.xml", `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="p"><application/></manifest>`)
			writeProjectFile(t, root, "src/main/res/layout/activity_main.xml", `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android">
    <Button android:id="@+id/submit" android:onClick="onSubmitClicked" />
</LinearLayout>
`)
			writeProjectFile(t, root, "src/main/kotlin/p/MainActivity.kt", strings.Replace(activity, "%s", tc.handler, 1))
			projectPath := root
			if tc.relative {
				t.Chdir(filepath.Dir(root))
				projectPath = filepath.Base(root)
			}
			out := analyzeAndroidResources(t, projectPath)
			// OnClick's message; OnClickResource flags the attribute either way.
			if got := strings.Contains(out, "has no matching method on"); got != tc.want {
				t.Fatalf("OnClick reported = %v, want %v: %s", got, tc.want, out)
			}
		})
	}
}

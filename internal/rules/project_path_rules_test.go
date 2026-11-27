package rules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// projectPathRuleFindings runs the registered rule id over a file written at
// root/rel, with root recorded as the dispatcher's project root, and returns
// how many findings it reports.
func projectPathRuleFindings(t *testing.T, id, root, rel, content string) int {
	t.Helper()
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == id {
			rule = r
			break
		}
	}
	if rule == nil {
		t.Fatalf("rule %s not registered", id)
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDispatcher([]*api.Rule{rule}, nil)
	d.SetProjectRoot(root)
	cols := d.Run(file)
	return cols.Len()
}

// A path heuristic reads the folders inside the project only: a checkout
// under a folder the heuristic excludes (debug/, tooling/, samples/,
// mocks/) still reports, and the same folder inside the project still
// excludes (#1834).
func TestRulePathHeuristicsIgnoreFoldersAboveProjectRoot(t *testing.T) {
	cases := []struct {
		rule, folder, inside, content string
	}{
		{
			rule: "HardcodedLocalhostUrl", folder: "debug", inside: "app/src/debug/kotlin/p/Api.kt",
			content: "package p\n\nval api = \"http://localhost:8080/api/v1\"\n",
		},
		{
			rule: "HardcodedEnvironmentName", folder: "debug", inside: "app/src/debug/kotlin/p/Api.kt",
			content: "package p\n\nfun apiConfig(environment: String): String = environment\n\nfun createApi(): String {\n    return apiConfig(\"staging\")\n}\n",
		},
		{
			rule: "PrintStackTraceInProduction", folder: "tooling", inside: "tooling/src/main/kotlin/p/UserService.kt",
			content: "package p\n\nimport timber.log.Timber\n\nclass UserService {\n    fun load() {\n        try {\n            doWork()\n        } catch (e: Exception) {\n            e.printStackTrace()\n        }\n    }\n\n    private fun doWork() {}\n}\n",
		},
		{
			rule: "PrintlnInProduction", folder: "samples", inside: "samples/app/src/main/kotlin/p/UserService.kt",
			content: "package p\n\nclass UserService {\n    fun load() {\n        println(\"loading\")\n    }\n}\n",
		},
		{
			rule: "OkHttpClientCreatedPerCall", folder: "scripts", inside: "scripts/src/main/kotlin/p/Client.kt",
			content: "package p\n\nimport okhttp3.OkHttpClient\n\nclass Client {\n    fun makeRequest(url: String) {\n        val client = OkHttpClient.Builder()\n            .connectTimeout(30, java.util.concurrent.TimeUnit.SECONDS)\n            .build()\n        val request = Request.Builder().url(url).build()\n        client.newCall(request).execute()\n    }\n}\n",
		},
		{
			rule: "TestOnlyImportInProduction", folder: "mocks", inside: "mocks/src/main/kotlin/p/ProdClass.kt",
			content: "package p\n\nimport io.mockk.mockk\n\nclass ProdClass {\n    val mock = mockk<String>()\n}\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.rule, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), tc.folder, "repo")
			if n := projectPathRuleFindings(t, tc.rule, root, "app/src/main/kotlin/p/Main.kt", tc.content); n == 0 {
				t.Errorf("%s reported nothing for a production file of a checkout under %s/", tc.rule, tc.folder)
			}
			if n := projectPathRuleFindings(t, tc.rule, root, tc.inside, tc.content); n != 0 {
				t.Errorf("%s reported %d findings for %s, which a %s/ folder inside the project excludes", tc.rule, n, tc.inside, tc.folder)
			}
		})
	}
}

// A configured test marker spelled as an absolute path inside the project
// root still marks test sources once rules read root-relative paths, and a
// relative marker still matches only inside the project (#1834).
func TestConfiguredAbsoluteTestMarkerUnderMarkedRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "spec", "repo")
	scanner.InitTestPaths([]string{filepath.Join(root, "spec") + string(filepath.Separator)}, nil)
	t.Cleanup(func() { scanner.InitTestPaths(nil, nil) })

	const rule = "PrintStackTraceInProduction"
	content := "package p\n\nimport timber.log.Timber\n\nclass UserService {\n    fun load() {\n        try {\n            doWork()\n        } catch (e: Exception) {\n            e.printStackTrace()\n        }\n    }\n\n    private fun doWork() {}\n}\n"
	if n := projectPathRuleFindings(t, rule, root, "spec/kotlin/p/UserService.kt", content); n != 0 {
		t.Errorf("%s reported %d findings under the configured absolute test marker", rule, n)
	}
	if n := projectPathRuleFindings(t, rule, root, "app/src/main/kotlin/p/UserService.kt", content); n == 0 {
		t.Errorf("%s reported nothing for a production file", rule)
	}

	scanner.InitTestPaths([]string{"/spec/"}, nil)
	if n := projectPathRuleFindings(t, rule, root, "app/src/main/kotlin/p/UserService.kt", content); n == 0 {
		t.Error("the relative marker /spec/ matched the spec/ folder above the project root")
	}
	if n := projectPathRuleFindings(t, rule, root, "spec/kotlin/p/UserService.kt", content); n != 0 {
		t.Errorf("the relative marker /spec/ no longer marks a spec/ folder inside the project (%d findings)", n)
	}
}

// Configured non-production markers (nonProductionPathMarkers) spelled as an
// absolute path inside the project root keep matching root-relative paths;
// relative ones match only folders inside the project (#1834).
func TestProjectPathMarkers(t *testing.T) {
	root := filepath.Join(t.TempDir(), "tools", "repo")
	ctx := &api.Context{Facts: NewDispatcher(nil, nil).fileFacts}
	ctx.Facts.SetProjectRoot(root)
	// Screen.kt, not a name a built-in marker such as /debug matches, so only
	// the configured marker can match.
	inTools := projectPath(ctx, filepath.Join(root, "tools", "Screen.kt"))
	inApp := projectPath(ctx, filepath.Join(root, "app", "Screen.kt"))

	absolute := projectPathMarkers(ctx, []string{filepath.Join(root, "tools") + string(filepath.Separator)})
	if !composeDebugToolingPath(inTools, absolute) {
		t.Errorf("absolute marker %v did not match %s", absolute, inTools)
	}
	if composeDebugToolingPath(inApp, absolute) {
		t.Errorf("absolute marker %v matched %s", absolute, inApp)
	}
	relative := projectPathMarkers(ctx, []string{"/tools/"})
	if composeDebugToolingPath(inApp, relative) {
		t.Error("relative marker /tools/ matched the tools/ folder above the project root")
	}
	if !composeDebugToolingPath(inTools, relative) {
		t.Error("relative marker /tools/ no longer matches a tools/ folder inside the project")
	}
	if markers := []string{"/tools/"}; &projectPathMarkers(ctx, markers)[0] != &markers[0] {
		t.Error("unchanged markers were copied")
	}
}

// UseSdkSuppress reads only test sources, judged by the folders inside the
// project: a checkout under a test/ folder does not make its main sources
// test sources.
func TestUseSdkSuppressIgnoresTestFolderAboveProjectRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "test", "repo")
	content := "package p\n\nimport androidx.annotation.RequiresApi\nimport org.junit.Test\n\nclass ApiTest {\n    @Test\n    @RequiresApi(28)\n    fun usesNewApi() {}\n}\n"
	if n := projectPathRuleFindings(t, "UseSdkSuppress", root, "app/src/main/java/p/ApiTest.kt", content); n != 0 {
		t.Errorf("UseSdkSuppress reported %d findings for a main source of a checkout under test/", n)
	}
	if n := projectPathRuleFindings(t, "UseSdkSuppress", root, "app/src/test/java/p/ApiTest.kt", content); n == 0 {
		t.Error("UseSdkSuppress reported nothing for a test source")
	}
}

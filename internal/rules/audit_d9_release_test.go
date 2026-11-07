package rules_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditReleaseEngineering(t *testing.T) {
	runAuditCases(t, []auditCase{
		{"inverted debug logs error", "BuildConfigDebugInverted", `
package test
object BuildConfig { const val DEBUG = true }
fun f() { if (!BuildConfig.DEBUG) { Log.e("t", "m") } }
object Log { fun e(a: String, b: String) {} }
`, 0},
		{"inverted debug logs debug", "BuildConfigDebugInverted", `
package test
object BuildConfig { const val DEBUG = true }
fun f() { if (!BuildConfig.DEBUG) { Log.d("t", "m") } }
object Log { fun d(a: String, b: String) {} }
`, 1},
		{"inverted debug kotlin error", "BuildConfigDebugInverted", `
package test
object BuildConfig { const val DEBUG = true }
fun f() { if (!BuildConfig.DEBUG) { error("release only") } }
`, 0},
		{"env name config", "HardcodedEnvironmentName", `
package test
fun f() { loadConfig("staging") }
fun loadConfig(s: String) {}
`, 1},
		{"localhost url", "HardcodedLocalhostUrl", `
package test
const val U = "http://localhost:8080/api"
`, 1},
		{"localhost lookalike", "HardcodedLocalhostUrl", `
package test
const val U = "http://localhostel.example.com/api"
`, 0},
		{"toast debug", "DebugToastInProduction", `
package test
import android.widget.Toast
fun f(c: android.content.Context) { Toast.makeText(c, "Debug: x", Toast.LENGTH_SHORT).show() }
`, 1},
		{"toast testament", "DebugToastInProduction", `
package test
import android.widget.Toast
fun f(c: android.content.Context) { Toast.makeText(c, "Testament saved", Toast.LENGTH_SHORT).show() }
`, 0},
		{"println in main", "PrintlnInProduction", `
package test
fun main() { println("x") }
`, 0},
		{"println in class", "PrintlnInProduction", `
package test
class A { fun f() { println("x") } }
`, 1},
		{"printwriter println", "PrintlnInProduction", `
package test
class A { fun f(out: java.io.PrintWriter) { out.println("x") } }
`, 0},
		{"stacktrace without logging import", "PrintStackTraceInProduction", `
package test
fun f(e: Exception) { e.printStackTrace() }
`, 0},
		{"stacktrace with timber", "PrintStackTraceInProduction", `
package test
import timber.log.Timber
fun f(e: Exception) { e.printStackTrace() }
`, 1},
		{"commented import", "CommentedOutImport", `
package test
// import foo.Bar
class A
`, 1},
		{"commented import prose", "CommentedOutImport", `
package test
// import order matters here
class A
`, 0},
		{"junit import in prod", "TestOnlyImportInProduction", `
package test
import org.junit.Assert
class A
`, 1},
		{"hardcoded tag", "HardcodedLogTag", `
package test
import android.util.Log
class Foo { fun f() { Log.d("Foo", "x") } }
`, 1},
		{"hardcoded tag other", "HardcodedLogTag", `
package test
import android.util.Log
class Foo { fun f() { Log.d("Bar", "x") } }
`, 0},
	})
}

func TestAuditTimberPlantLocations(t *testing.T) {
	feature := `package com.example

import timber.log.Timber

fun logIt() {
    Timber.d("hello")
}
`
	for name, plant := range map[string]string{
		"application helper": `package com.example

import android.app.Application
import timber.log.Timber

class App : Application() {
    override fun onCreate() {
        super.onCreate()
        setupLogging()
    }

    private fun setupLogging() {
        Timber.plant(Timber.DebugTree())
    }
}
`,
		"startup initializer": `package com.example

import android.content.Context
import androidx.startup.Initializer
import timber.log.Timber

class LoggingInitializer : Initializer<Unit> {
    override fun create(context: Context) {
        Timber.plant(Timber.DebugTree())
    }

    override fun dependencies(): List<Class<out Initializer<*>>> = emptyList()
}
`,
	} {
		t.Run(name, func(t *testing.T) {
			findings := runTimberTreeNotPlantedRule(t, map[string]string{
				"app/src/main/java/com/example/Plant.kt":   plant,
				"app/src/main/java/com/example/Feature.kt": feature,
			}, nil)
			if len(findings) != 0 {
				t.Fatalf("expected no findings, got %d", len(findings))
			}
		})
	}
}

func TestAuditConventionPluginChaining(t *testing.T) {
	root := t.TempDir()
	writeModuleFile(t, filepath.Join(root, "settings.gradle.kts"), `include(":app")`)
	writeModuleFile(t, filepath.Join(root, "app", "build.gradle.kts"), `plugins { id("app-conventions") }`)
	writeModuleFile(t, filepath.Join(root, "build-logic", "src", "main", "kotlin", "app-conventions.gradle.kts"), `plugins { id("base-conventions") }`)
	writeModuleFile(t, filepath.Join(root, "build-logic", "src", "main", "kotlin", "base-conventions.gradle.kts"), `plugins { id("java") }`)
	writeModuleFile(t, filepath.Join(root, "build-logic", "src", "main", "kotlin", "orphan-conventions.gradle.kts"), `plugins { id("java") }`)
	findings := runConventionPluginDeadCodeRule(t, root)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "orphan-conventions") {
		t.Fatalf("expected only the orphan plugin to be reported, got %v", findings)
	}
}

func TestAuditCommentedOutCodeBlock(t *testing.T) {
	runAuditCases(t, []auditCase{
		{"prose with keywords", "CommentedOutCodeBlock", `
package test
// if the user cancels we keep the old value
// when this runs the cache is already warm
// return the cached value if there is one
// the default = 5 is arbitrary
fun f() {}
`, 0},
		{"real code", "CommentedOutCodeBlock", `
package test
// val x = compute()
// if (x > 3) {
//   println(x)
// }
fun f() {}
`, 1},
		{"commented assignments", "CommentedOutCodeBlock", `
package test
// count = count + 1
// name = "x"
// total += 2
fun f() {}
`, 1},
	})
}

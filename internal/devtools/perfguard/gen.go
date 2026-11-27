package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// genSpec sizes a synthetic Android-shaped project. The shape follows the
// projects where krit's project-scope phases got slow: many Gradle modules,
// each with a manifest declaring activities, services, receivers and a
// provider, Kotlin component classes behind shared base classes, Compose
// functions, plain classes, string resources and layouts.
type genSpec struct {
	Modules        int
	FilesPerModule int
	ClassesPerFile int
}

func (s genSpec) classes() int { return s.Modules * s.FilesPerModule * s.ClassesPerFile }

// generate writes the project under dir (which must not exist or be empty).
// Output is deterministic: the same spec always produces the same bytes.
func generate(dir string, spec genSpec) error {
	if spec.Modules < 1 || spec.FilesPerModule < 1 || spec.ClassesPerFile < 1 {
		return fmt.Errorf("modules, files and classes must be >= 1 (got %+v)", spec)
	}
	w := &treeWriter{root: dir}
	var settings strings.Builder
	settings.WriteString("rootProject.name = \"perfguard-synthetic\"\n\ninclude(\":app\")\n")
	for m := 0; m < spec.Modules; m++ {
		fmt.Fprintf(&settings, "include(\":feature%d\")\n", m)
	}
	w.file("settings.gradle.kts", settings.String())
	w.file("build.gradle.kts", "plugins {\n    id(\"com.android.application\") version \"8.7.0\" apply false\n    id(\"com.android.library\") version \"8.7.0\" apply false\n    kotlin(\"android\") version \"2.1.0\" apply false\n}\n")
	w.file("gradle/libs.versions.toml", "[versions]\nkotlin = \"2.1.0\"\ncompose-bom = \"2024.10.00\"\n\n[libraries]\ncompose-bom = { module = \"androidx.compose:compose-bom\", version.ref = \"compose-bom\" }\ncompose-runtime = { module = \"androidx.compose.runtime:runtime\" }\n")

	var appDeps strings.Builder
	for m := 0; m < spec.Modules; m++ {
		fmt.Fprintf(&appDeps, "    implementation(project(\":feature%d\"))\n", m)
		genModule(w, spec, m)
	}
	w.file("app/build.gradle.kts", androidBuild("com.android.application", "com.example.app", appDeps.String()))
	w.file("app/src/main/AndroidManifest.xml", `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <application android:label="@string/app_name">
        <activity android:name="com.example.app.MainActivity" android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
    </application>
</manifest>
`)
	w.file("app/src/main/kotlin/com/example/app/MainActivity.kt", `package com.example.app

import android.app.Activity
import android.os.Bundle

class MainActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.main)
    }
}
`)
	w.file("app/src/main/res/values/strings.xml", "<resources>\n    <string name=\"app_name\">Synthetic</string>\n</resources>\n")
	w.file("app/src/main/res/layout/main.xml", layoutXML(0))
	return w.err
}

func androidBuild(plugin, namespace, deps string) string {
	return fmt.Sprintf(`plugins {
    id("%s")
    kotlin("android")
}

android {
    namespace = "%s"
    compileSdk = 35
    defaultConfig {
        minSdk = 24
        targetSdk = 35
    }
}

dependencies {
    implementation(platform(libs.compose.bom))
    implementation(libs.compose.runtime)
%s}
`, plugin, namespace, deps)
}

func genModule(w *treeWriter, spec genSpec, m int) {
	mod := fmt.Sprintf("feature%d", m)
	pkg := fmt.Sprintf("com.example.f%d", m)
	pkgDir := strings.ReplaceAll(pkg, ".", "/")
	w.file(mod+"/build.gradle.kts", androidBuild("com.android.library", pkg, ""))

	var manifest strings.Builder
	manifest.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\">\n    <application>\n")
	fmt.Fprintf(&manifest, "        <provider android:name=\"%s.Provider%d\" android:authorities=\"%s.provider\" android:exported=\"true\" android:readPermission=\"%s.READ\" />\n", pkg, m, pkg, pkg)

	// Base classes shared by the module's components, so supertype checks
	// have to walk a chain instead of matching the platform class directly.
	w.file(fmt.Sprintf("%s/src/main/kotlin/%s/Bases.kt", mod, pkgDir), fmt.Sprintf(`package %[1]s

import android.app.Activity
import android.app.Service
import android.content.BroadcastReceiver
import android.content.ContentProvider
import android.content.ContentValues
import android.content.Context
import android.content.Intent
import android.database.Cursor
import android.net.Uri
import android.os.IBinder

abstract class BaseActivity%[2]d : Activity() {
    protected fun forward(intent: Intent) {
        startActivity(intent)
    }
}

abstract class BaseService%[2]d : Service() {
    override fun onBind(intent: Intent?): IBinder? = null
}

abstract class BaseReceiver%[2]d : BroadcastReceiver()

class Provider%[2]d : ContentProvider() {
    override fun onCreate(): Boolean = true
    override fun query(uri: Uri, projection: Array<String>?, selection: String?, selectionArgs: Array<String>?, sortOrder: String?): Cursor? = null
    override fun getType(uri: Uri): String? = null
    override fun insert(uri: Uri, values: ContentValues?): Uri? = null
    override fun delete(uri: Uri, selection: String?, selectionArgs: Array<String>?): Int = 0
    override fun update(uri: Uri, values: ContentValues?, selection: String?, selectionArgs: Array<String>?): Int = 0
}
`, pkg, m))

	var strs strings.Builder
	strs.WriteString("<resources>\n")
	for f := 0; f < spec.FilesPerModule; f++ {
		var src strings.Builder
		fmt.Fprintf(&src, `package %s

import android.content.Context
import android.content.Intent
import android.os.Bundle
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue

`, pkg)
		for c := 0; c < spec.ClassesPerFile; c++ {
			id := fmt.Sprintf("M%dF%dC%d", m, f, c)
			switch c % 6 {
			case 0:
				fmt.Fprintf(&manifest, "        <activity android:name=\"%s.Screen%s\" android:exported=\"%t\" />\n", pkg, id, c%12 == 0)
				fmt.Fprintf(&src, `class Screen%[1]s : BaseActivity%[2]d() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val next = intent.getParcelableExtra<Intent>("next")
        if (next != null) {
            forward(next)
        }
        title = getString(R.string.title_%[3]s)
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
    }
}

`, id, m, strings.ToLower(id))
				fmt.Fprintf(&strs, "    <string name=\"title_%s\">Screen %s</string>\n", strings.ToLower(id), id)
			case 1:
				fmt.Fprintf(&manifest, "        <service android:name=\"%s.Worker%s\" android:exported=\"false\" />\n", pkg, id)
				fmt.Fprintf(&src, `class Worker%[1]s : BaseService%[2]d() {
    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val payload = intent?.getStringExtra("payload") ?: return START_NOT_STICKY
        process(payload)
        return START_STICKY
    }

    private fun process(payload: String) {
        val parts = payload.split(",").map { it.trim() }.filter { it.isNotEmpty() }
        if (parts.size > 3) {
            stopSelf()
        }
    }
}

`, id, m)
			case 2:
				fmt.Fprintf(&manifest, "        <receiver android:name=\"%s.Receiver%s\" android:exported=\"true\" />\n", pkg, id)
				fmt.Fprintf(&src, `class Receiver%[1]s : BaseReceiver%[2]d() {
    override fun onReceive(context: Context, intent: Intent) {
        val target = intent.getParcelableExtra<Intent>("target") ?: return
        context.startActivity(target)
    }
}

`, id, m)
			case 3:
				fmt.Fprintf(&src, `@Composable
fun Counter%[1]s(label: String, onClick: () -> Unit) {
    var count by remember { mutableStateOf(0) }
    val doubled = remember(count) { count * 2 }
    if (label.isNotEmpty() && doubled >= 0) {
        onClick()
    }
}

`, id)
			case 4:
				fmt.Fprintf(&src, `data class Model%[1]s(val id: Long, val name: String, val tags: List<String> = emptyList()) {
    fun describe(): String = when {
        tags.isEmpty() -> name
        tags.size == 1 -> "$name (${tags.first()})"
        else -> "$name (${tags.size} tags)"
    }
}

`, id)
			default:
				fmt.Fprintf(&src, `class Repository%[1]s(private val context: Context) {
    private val cache = mutableMapOf<Long, Model%[2]s>()

    fun load(id: Long): Model%[2]s? {
        cache[id]?.let { return it }
        val prefs = context.getSharedPreferences("repo_%[3]s", Context.MODE_PRIVATE)
        val name = prefs.getString("name_$id", null) ?: return null
        val model = Model%[2]s(id, name)
        cache[id] = model
        return model
    }

    fun clear() {
        cache.clear()
    }
}

`, id, fmt.Sprintf("M%dF%dC%d", m, f, c-1), strings.ToLower(id))
			}
		}
		w.file(fmt.Sprintf("%s/src/main/kotlin/%s/File%d.kt", mod, pkgDir, f), src.String())
		if f%4 == 0 {
			w.file(fmt.Sprintf("%s/src/main/res/layout/screen_%d_%d.xml", mod, m, f), layoutXML(f))
		}
	}
	strs.WriteString("</resources>\n")
	manifest.WriteString("    </application>\n</manifest>\n")
	w.file(mod+"/src/main/AndroidManifest.xml", manifest.String())
	w.file(mod+"/src/main/res/values/strings.xml", strs.String())
}

func layoutXML(n int) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent"
    android:layout_height="match_parent"
    android:orientation="vertical">
    <TextView
        android:id="@+id/title_%d"
        android:layout_width="wrap_content"
        android:layout_height="wrap_content"
        android:text="Title" />
    <Button
        android:id="@+id/action_%d"
        android:layout_width="wrap_content"
        android:layout_height="wrap_content" />
</LinearLayout>
`, n, n)
}

type treeWriter struct {
	root string
	err  error
}

func (w *treeWriter) file(rel, content string) {
	if w.err != nil {
		return
	}
	path := filepath.Join(w.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		w.err = err
		return
	}
	w.err = os.WriteFile(path, []byte(content), 0o644)
}

// generateJVM writes a JVM-only multi-module project for timing FIR and the
// type oracle (#1729): a `core` module and spec.Modules feature modules that
// depend on it, with an exported-style Gradle model under .krit/gradle-model
// so krit compiles each source set with its project dependencies. It needs no
// Android SDK, no Gradle run and no network: the classpath is the Kotlin
// standard library krit-fir brings. Output is deterministic for a given dir.
func generateJVM(dir string, spec genSpec) error {
	if spec.Modules < 1 || spec.FilesPerModule < 1 || spec.ClassesPerFile < 1 {
		return fmt.Errorf("modules, files and classes must be >= 1 (got %+v)", spec)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	w := &treeWriter{root: abs}
	var settings strings.Builder
	settings.WriteString("rootProject.name = \"perfguard-jvm\"\n\ninclude(\":core\")\n")
	for m := 0; m < spec.Modules; m++ {
		fmt.Fprintf(&settings, "include(\":feature%d\")\n", m)
	}
	w.file("settings.gradle.kts", settings.String())

	type project struct {
		Path       string           `json:"path"`
		Dir        string           `json:"dir"`
		SourceSets []map[string]any `json:"sourceSets"`
	}
	sourceSet := func(name string, deps ...string) []map[string]any {
		return []map[string]any{{"name": "main", "kind": "main", "platform": "jvm", "projectDeps": deps,
			"sourceDirs": []string{filepath.Join(abs, name, "src", "main", "kotlin")}}}
	}
	projects := []project{{Path: ":core", Dir: filepath.Join(abs, "core"), SourceSets: sourceSet("core")}}
	w.file("core/build.gradle.kts", "plugins {\n    kotlin(\"jvm\")\n}\n")
	for f := 0; f < spec.FilesPerModule; f++ {
		w.file(fmt.Sprintf("core/src/main/kotlin/com/example/core/Core%d.kt", f), jvmCoreFile(f, spec.ClassesPerFile))
	}
	for m := 0; m < spec.Modules; m++ {
		mod := fmt.Sprintf("feature%d", m)
		w.file(mod+"/build.gradle.kts", "plugins {\n    kotlin(\"jvm\")\n}\n\ndependencies {\n    implementation(project(\":core\"))\n}\n")
		for f := 0; f < spec.FilesPerModule; f++ {
			w.file(fmt.Sprintf("%s/src/main/kotlin/com/example/f%d/Feature%d.kt", mod, m, f), jvmFeatureFile(m, f, spec.ClassesPerFile))
		}
		projects = append(projects, project{Path: ":" + mod, Dir: filepath.Join(abs, mod), SourceSets: sourceSet(mod, ":core")})
	}
	model, err := json.MarshalIndent(map[string]any{"schema": 2, "rootDir": abs, "projects": projects}, "", "  ")
	if err != nil {
		return err
	}
	w.file(".krit/gradle-model/model.json", string(model)+"\n")
	return w.err
}

// jvmCoreFile declares an open base class, an interface and a sealed result
// per class slot, for the feature modules to extend and call.
func jvmCoreFile(f, classes int) string {
	var b strings.Builder
	b.WriteString("package com.example.core\n\n")
	for c := 0; c < classes; c++ {
		fmt.Fprintf(&b, `interface Source%[1]d_%[2]d {
    fun load(id: Int): Result%[1]d_%[2]d
}

sealed interface Result%[1]d_%[2]d {
    data class Ok(val value: String) : Result%[1]d_%[2]d
    data class Failed(val code: Int) : Result%[1]d_%[2]d
}

open class Base%[1]d_%[2]d(protected val name: String) {
    open fun describe(): String = "base " + name
    fun length(): Int = name.length
}

`, f, c)
	}
	return b.String()
}

// jvmFeatureFile extends and calls core's declarations: overrides, a when over
// a sealed type, lambdas, nullable chains and a suspend function, so the
// checkers resolve across the module boundary.
func jvmFeatureFile(m, f, classes int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package com.example.f%d\n\nimport com.example.core.*\n\n", m)
	for c := 0; c < classes; c++ {
		fmt.Fprintf(&b, `class Feature%[1]d_%[2]d_%[3]d(name: String) : Base%[2]d_%[3]d(name), Source%[2]d_%[3]d {
    private val cache = mutableMapOf<Int, String>()

    override fun describe(): String = "feature %[1]d " + super.describe()

    override fun load(id: Int): Result%[2]d_%[3]d {
        val hit = cache[id]
        return if (hit != null) Result%[2]d_%[3]d.Ok(hit) else Result%[2]d_%[3]d.Failed(id)
    }

    fun render(id: Int): String = when (val r = load(id)) {
        is Result%[2]d_%[3]d.Ok -> r.value.uppercase()
        is Result%[2]d_%[3]d.Failed -> "error " + r.code
    }

    fun total(ids: List<Int>): Int = ids.map { render(it).length }.filter { it > %[3]d }.sum() + length()

    suspend fun refresh(id: Int): String? = cache[id]?.trim()?.takeIf { it.isNotEmpty() }
}

`, m, f, c)
	}
	return b.String()
}

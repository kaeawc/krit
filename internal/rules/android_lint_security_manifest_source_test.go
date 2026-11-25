package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Expected findings come from running Android Lint 32.4.1 on the same projects.

// manifestSourceFindings lays out an app module (src/main/AndroidManifest.xml and sources under src/main/java), runs
// a manifest-source rule over it, and returns "path:line:col message" rows relative to src/main/java (or
// "AndroidManifest.xml:..."), sorted.
func manifestSourceFindings(t *testing.T, ruleID, manifestXML string, sources map[string]string) []string {
	t.Helper()
	main := filepath.Join(t.TempDir(), "app", "src", "main")
	m := parseManifestAt(t, filepath.Join(main, "AndroidManifest.xml"), manifestXML)
	var files []*scanner.File
	for name, body := range sources {
		path := filepath.Join(main, "java", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		var f *scanner.File
		var err error
		if strings.HasSuffix(name, ".java") {
			f, err = scanner.ParseJavaFile(context.Background(), path)
		} else {
			f, err = scanner.ParseFile(context.Background(), path)
		}
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == ruleID {
			rule = r
		}
	}
	if rule == nil {
		t.Fatalf("rule %s not registered", ruleID)
	}
	d := rules.NewDispatcher([]*api.Rule{rule}, nil)
	cols, ok := d.RunManifestSource([]*rules.Manifest{m}, files)
	if !ok {
		t.Fatal("RunManifestSource recovered a panic")
	}
	var out []string
	for _, f := range cols.Findings() {
		rel, err := filepath.Rel(filepath.Join(main, "java"), f.File)
		if err != nil || strings.HasPrefix(rel, "..") {
			rel = filepath.Base(f.File)
		}
		out = append(out, fmt.Sprintf("%s:%d:%d %s", filepath.ToSlash(rel), f.Line, f.Col, f.Message))
	}
	sort.Strings(out)
	return out
}

// parseManifestAt writes the manifest to path and parses it into the rule-facing model.
func parseManifestAt(t *testing.T, path, xml string) *rules.Manifest {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	m := parseManifestForTest(t, xml)
	if err := os.WriteFile(path, []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	m.Path = path
	return m
}

func assertFindings(t *testing.T, got, want []string) {
	t.Helper()
	want = append([]string(nil), want...)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("findings:\n%s\nwant (Lint 32.4.1):\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestProviderReadPermissionOnlyLintParity(t *testing.T) {
	manifest := `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.prov">
    <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34" />
    <application>
        <provider android:name=".WriteProvider" android:authorities="a1" android:readPermission="test.READ" android:exported="true" />
        <provider android:name="com.prov.StubProvider" android:authorities="a2" android:readPermission="test.READ" />
        <provider android:name=".WriteProvider2" android:authorities="a3" android:readPermission="test.READ" android:writePermission="test.WRITE" />
        <provider android:name=".WriteProvider3" android:authorities="a4" android:readPermission="test.READ" android:permission="test.ALL" />
        <provider android:name=".JavaProvider" android:authorities="a5" android:readPermission="test.READ" />
        <provider android:name=".SubProvider" android:authorities="a6" android:readPermission="test.READ" />
        <provider android:name=".EmptyBodies" android:authorities="a7" android:readPermission="test.READ" />
        <provider android:name=".WrongArity" android:authorities="a8" android:readPermission="test.READ" />
        <provider android:name=".Outer$Nested" android:authorities="a9" android:readPermission="test.READ" />
    </application>
</manifest>
`
	sources := map[string]string{
		"com/prov/JavaProvider.java": `package com.prov;

import android.content.ContentValues;
import android.net.Uri;

public class JavaProvider extends BaseProvider {
    @Override public Uri insert(Uri uri, ContentValues values) { throw new UnsupportedOperationException(); }
    @Override public int update(Uri uri, ContentValues values, String s, String[] a) { return 0; }
    @Override public int delete(Uri uri, String s, String[] a) { int n = 1; return n; }
}
`,
		"com/prov/Providers.kt": `package com.prov

import android.content.ContentProvider
import android.content.ContentValues
import android.database.Cursor
import android.net.Uri

abstract class BaseProvider : ContentProvider() {
    override fun onCreate() = true
    override fun query(uri: Uri, p: Array<String>?, s: String?, a: Array<String>?, o: String?): Cursor? = null
    override fun getType(uri: Uri): String? = null
}

class WriteProvider : BaseProvider() {
    override fun insert(uri: Uri, values: ContentValues?): Uri? {
        println("x")
        return uri
    }
    override fun update(uri: Uri, values: ContentValues?, s: String?, a: Array<String>?): Int = 0
    override fun delete(uri: Uri, s: String?, a: Array<String>?): Int {
        return compute()
    }
    fun compute() = 1
}

class StubProvider : BaseProvider() {
    override fun insert(uri: Uri, values: ContentValues?): Uri? = null
    override fun update(uri: Uri, values: ContentValues?, s: String?, a: Array<String>?): Int = TODO()
    override fun delete(uri: Uri, s: String?, a: Array<String>?): Int = throw UnsupportedOperationException()
}

class WriteProvider2 : WriteProvider3()
open class WriteProvider3 : BaseProvider() {
    override fun insert(uri: Uri, values: ContentValues?): Uri? = uri
    override fun update(uri: Uri, values: ContentValues?, s: String?, a: Array<String>?): Int = 0
    override fun delete(uri: Uri, s: String?, a: Array<String>?): Int = 0
}

class SubProvider : WriteProvider3()

class EmptyBodies : BaseProvider() {
    override fun insert(uri: Uri, values: ContentValues?): Uri? { return null }
    override fun update(uri: Uri, values: ContentValues?, s: String?, a: Array<String>?): Int { return 0 }
    override fun delete(uri: Uri, s: String?, a: Array<String>?): Int { error("no") }
}

class WrongArity : BaseProvider() {
    override fun insert(uri: Uri, values: ContentValues?): Uri? = null
    override fun update(uri: Uri, values: ContentValues?, s: String?, a: Array<String>?): Int = 0
    override fun delete(uri: Uri, s: String?, a: Array<String>?): Int = 0
    fun insert(uri: Uri, values: ContentValues?, extra: Int): Uri? = uri
    fun delete(uri: Uri, extras: android.os.Bundle?): Int = 1
}

class Outer {
    class Nested : BaseProvider() {
        override fun insert(uri: Uri, values: ContentValues?): Uri? = uri
        override fun update(uri: Uri, values: ContentValues?, s: String?, a: Array<String>?): Int = 0
        override fun delete(uri: Uri, s: String?, a: Array<String>?): Int = 0
    }
}
`,
	}
	want := []string{
		"AndroidManifest.xml:4:74 com.prov.WriteProvider implements {`insert`, `delete`} write APIs but does not protect them with a permission. Update the <provider> tag to use android:permission or android:writePermission",
		"AndroidManifest.xml:8:73 com.prov.JavaProvider implements {`delete`} write APIs but does not protect them with a permission. Update the <provider> tag to use android:permission or android:writePermission",
		"AndroidManifest.xml:12:73 com.prov.Outer.Nested implements {`insert`} write APIs but does not protect them with a permission. Update the <provider> tag to use android:permission or android:writePermission",
	}
	assertFindings(t, manifestSourceFindings(t, "ProviderReadPermissionOnly", manifest, sources), want)
}

func TestUnsafeImplicitIntentLaunchLintParity(t *testing.T) {
	manifest := `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example">
    <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34" />
    <application>
        <activity android:name=".InternalActivity" android:exported="false">
            <intent-filter>
                <action android:name="com.example.INTERNAL_VIEW" />
                <category android:name="android.intent.category.DEFAULT" />
            </intent-filter>
        </activity>
        <activity android:name=".PublicActivity" android:exported="true">
            <intent-filter>
                <action android:name="com.example.PUBLIC_VIEW" />
            </intent-filter>
        </activity>
        <receiver android:name=".InternalReceiver" android:exported="false">
            <intent-filter>
                <action android:name="com.example.INTERNAL_PING" />
            </intent-filter>
        </receiver>
        <service android:name=".InternalService" android:exported="false">
            <intent-filter>
                <action android:name="com.example.INTERNAL_SERVICE" />
            </intent-filter>
        </service>
        <activity android:name=".ImplicitDefault">
            <intent-filter>
                <action android:name="com.example.DEFAULT_EXPORT" />
            </intent-filter>
        </activity>
    </application>
</manifest>
`
	sources := map[string]string{
		"com/example/JSend.java": `package com.example;

import android.content.Context;
import android.content.Intent;

class JSend {
    void a(Context context) { context.startActivity(new Intent("com.example.INTERNAL_VIEW")); }
    void b(Context context) {
        Intent i = new Intent();
        i.setAction("com.example.INTERNAL_PING");
        context.sendBroadcast(i);
    }
}
`,
		"com/example/Senders.kt": `package com.example

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter

const val ACTION_CONST = "com.example.INTERNAL_VIEW"

class Senders(private val context: Context) {
    fun a() { context.startActivity(Intent("com.example.INTERNAL_VIEW")) }
    fun b() { context.startActivity(Intent("com.example.PUBLIC_VIEW")) }
    fun c() { context.sendBroadcast(Intent("com.example.INTERNAL_PING")) }
    fun d() { context.startActivity(Intent("com.example.INTERNAL_PING")) }
    fun e() { context.startService(Intent("com.example.INTERNAL_SERVICE")) }
    fun f() {
        val i = Intent()
        i.action = "com.example.INTERNAL_VIEW"
        context.startActivity(i)
    }
    fun g() {
        val i = Intent("com.example.INTERNAL_VIEW")
        i.setPackage(context.packageName)
        context.startActivity(i)
    }
    fun h() { context.startActivity(Intent(ACTION_CONST)) }
    fun i() {
        val i = Intent()
        i.setAction("com.example.INTERNAL_PING")
        context.sendBroadcast(i)
    }
    fun j() { context.startActivity(Intent("com.example.DEFAULT_EXPORT")) }
    fun k() { context.sendBroadcast(Intent("com.example.DYNAMIC")) }
    fun l(r: BroadcastReceiver) {
        context.registerReceiver(r, IntentFilter("com.example.DYNAMIC"), Context.RECEIVER_NOT_EXPORTED)
    }
    fun m() { context.sendBroadcast(Intent("com.example.DYNAMIC2")) }
    fun n(r: BroadcastReceiver) {
        context.registerReceiver(r, IntentFilter("com.example.DYNAMIC2"), Context.RECEIVER_EXPORTED)
    }
    fun o() { context.startActivity(Intent("com.example.INTERNAL_VIEW").setClassName("com.example", "x")) }
    fun p() {
        val intent = Intent("com.example.INTERNAL_VIEW")
        helper(intent)
    }
    fun helper(i: Intent) { context.startActivity(i) }
    fun q() { context.sendBroadcastAsUser(Intent("com.example.INTERNAL_PING"), android.os.Process.myUserHandle()) }
    fun r() { context.startActivity(Intent("com.example.INTERNAL_VIEW", android.net.Uri.EMPTY)) }
}

class ActivitySender : android.app.Activity() {
    fun s() { startActivity(Intent("com.example.INTERNAL_VIEW")) }
    fun t() { startActivityForResult(Intent("com.example.INTERNAL_VIEW"), 1) }
}
`,
	}
	want := []string{
		"com/example/JSend.java:7:53 The intent action `com.example.INTERNAL_VIEW (used to start an activity)` matches the intent filter of a non-exported component `com.example.InternalActivity` from a manifest. If you are trying to invoke this specific component via the action then you should make the intent explicit by calling `Intent.set{Component,Class,ClassName}`.",
		"com/example/JSend.java:10:9 The intent action `com.example.INTERNAL_PING (used to send a broadcast)` matches the intent filter of a non-exported component `com.example.InternalReceiver` from a manifest. If you are trying to invoke this specific component via the action then you should make the intent explicit by calling `Intent.set{Component,Class,ClassName}`.",
		"com/example/Senders.kt:11:37 The intent action `com.example.INTERNAL_VIEW (used to start an activity)` matches the intent filter of a non-exported component `com.example.InternalActivity` from a manifest. If you are trying to invoke this specific component via the action then you should make the intent explicit by calling `Intent.set{Component,Class,ClassName}`.",
		"com/example/Senders.kt:13:37 The intent action `com.example.INTERNAL_PING (used to send a broadcast)` matches the intent filter of a non-exported component `com.example.InternalReceiver` from a manifest. If you are trying to invoke this specific component via the action then you should make the intent explicit by calling `Intent.set{Component,Class,ClassName}`.",
		"com/example/Senders.kt:18:9 The intent action `com.example.INTERNAL_VIEW (used to start an activity)` matches the intent filter of a non-exported component `com.example.InternalActivity` from a manifest. If you are trying to invoke this specific component via the action then you should make the intent explicit by calling `Intent.set{Component,Class,ClassName}`.",
		"com/example/Senders.kt:26:37 The intent action `com.example.INTERNAL_VIEW (used to start an activity)` matches the intent filter of a non-exported component `com.example.InternalActivity` from a manifest. If you are trying to invoke this specific component via the action then you should make the intent explicit by calling `Intent.set{Component,Class,ClassName}`.",
		"com/example/Senders.kt:29:9 The intent action `com.example.INTERNAL_PING (used to send a broadcast)` matches the intent filter of a non-exported component `com.example.InternalReceiver` from a manifest. If you are trying to invoke this specific component via the action then you should make the intent explicit by calling `Intent.set{Component,Class,ClassName}`.",
		"com/example/Senders.kt:32:37 The intent action `com.example.DEFAULT_EXPORT (used to start an activity)` matches the intent filter of a non-exported component `com.example.ImplicitDefault` from a manifest. If you are trying to invoke this specific component via the action then you should make the intent explicit by calling `Intent.set{Component,Class,ClassName}`.",
		"com/example/Senders.kt:33:37 The intent action `com.example.DYNAMIC (used to send a broadcast)` matches the intent filter of a non-exported receiver, registered via a call to `Context.registerReceiver`, or similar. If you are trying to invoke this specific receiver via the action then you should use `Intent.setPackage(<APPLICATION_ID>)`.",
		"com/example/Senders.kt:47:43 The intent action `com.example.INTERNAL_PING (used to send a broadcast)` matches the intent filter of a non-exported component `com.example.InternalReceiver` from a manifest. If you are trying to invoke this specific component via the action then you should make the intent explicit by calling `Intent.set{Component,Class,ClassName}`.",
		"com/example/Senders.kt:48:37 The intent action `com.example.INTERNAL_VIEW (used to start an activity)` matches the intent filter of a non-exported component `com.example.InternalActivity` from a manifest. If you are trying to invoke this specific component via the action then you should make the intent explicit by calling `Intent.set{Component,Class,ClassName}`.",
		"com/example/Senders.kt:52:29 The intent action `com.example.INTERNAL_VIEW (used to start an activity)` matches the intent filter of a non-exported component `com.example.InternalActivity` from a manifest. If you are trying to invoke this specific component via the action then you should make the intent explicit by calling `Intent.set{Component,Class,ClassName}`.",
	}
	assertFindings(t, manifestSourceFindings(t, "UnsafeImplicitIntentLaunch", manifest, sources), want)
}

func TestUnsafeIntentLaunchLintParity(t *testing.T) {
	manifest := `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example">
    <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34" />
    <application>
        <activity android:name=".ExportedActivity" android:exported="true" />
        <activity android:name=".FilterActivity">
            <intent-filter><action android:name="com.example.OPEN" /></intent-filter>
        </activity>
        <activity android:name=".PrivateActivity" android:exported="false" />
        <activity android:name=".SignatureActivity" android:exported="true" android:permission="com.example.SIGNATURE_PERM" />
        <activity android:name=".NormalPermActivity" android:exported="true" android:permission="android.permission.INTERNET" />
        <service android:name=".ExportedService" android:exported="true" />
        <receiver android:name=".ExportedReceiver" android:exported="true" />
        <activity android:name=".JavaActivity" android:exported="true" />
    </application>
</manifest>
`
	sources := map[string]string{
		"com/example/A.kt": `package com.example

import android.app.Activity
import android.app.PendingIntent
import android.app.Service
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.os.Bundle
import android.os.IBinder

class ExportedActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val forward = intent.getParcelableExtra<Intent>("forward")
        startActivity(forward)
        val viaGetter = getIntent().getParcelableExtra<Intent>("second")
        startActivity(viaGetter)
        startActivity(intent)
        startActivity(getIntent())
        val uriIntent = Intent.parseUri(intent.getStringExtra("uri"), 0)
        startActivity(uriIntent)
        helper(intent.getParcelableExtra<Intent>("h")!!)
        val p = PendingIntent.getActivity(this, 0, intent.getParcelableExtra<Intent>("pi")!!, PendingIntent.FLAG_IMMUTABLE)
        setResult(RESULT_OK, intent.getParcelableExtra<Intent>("res"))
    }
    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        startActivity(intent)
    }
    private fun helper(i: Intent) { startActivity(i) }
}

class FilterActivity : Activity() {
    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        startActivity(intent)
    }
}

class PrivateActivity : Activity() {
    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        startActivity(intent)
    }
}

class SignatureActivity : Activity() {
    override fun onNewIntent(intent: Intent) { startActivity(intent) }
}

class NormalPermActivity : Activity() {
    override fun onNewIntent(intent: Intent) { startActivity(intent) }
}

class ExportedService : Service() {
    override fun onBind(intent: Intent?): IBinder? = null
    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val next = intent?.getParcelableExtra<Intent>("next")
        startService(next)
        return START_NOT_STICKY
    }
}

class ExportedReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action == Intent.ACTION_BOOT_COMPLETED) {
            context.startActivity(intent)
        }
        context.sendBroadcast(intent)
    }
}

class NotInManifest : Activity() {
    override fun onNewIntent(intent: Intent) { startActivity(intent) }
}
`,
		"com/example/JavaActivity.java": `package com.example;

import android.app.Activity;
import android.content.Intent;
import android.os.Bundle;

public class JavaActivity extends Activity {
    @Override protected void onCreate(Bundle b) {
        super.onCreate(b);
        Intent forward = getIntent().getParcelableExtra("forward");
        startActivity(forward);
        Intent copy = new Intent(getIntent());
        startActivity(copy);
    }
    @Override protected void onNewIntent(Intent intent) {
        Intent next = intent.getParcelableExtra("n");
        if (next != null) startActivity(next);
    }
}
`,
	}
	want := []string{
		"com/example/A.kt:15:23 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.ExportedActivity. You could either make the component com.example.ExportedActivity protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/A.kt:17:25 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.ExportedActivity. You could either make the component com.example.ExportedActivity protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/A.kt:19:23 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.ExportedActivity. You could either make the component com.example.ExportedActivity protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/A.kt:20:23 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.ExportedActivity. You could either make the component com.example.ExportedActivity protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/A.kt:21:25 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.ExportedActivity. You could either make the component com.example.ExportedActivity protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/A.kt:23:16 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.ExportedActivity. You could either make the component com.example.ExportedActivity protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/A.kt:24:52 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.ExportedActivity. You could either make the component com.example.ExportedActivity protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/A.kt:25:30 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.ExportedActivity. You could either make the component com.example.ExportedActivity protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/A.kt:27:30 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.ExportedActivity. You could either make the component com.example.ExportedActivity protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/A.kt:35:30 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.FilterActivity. You could either make the component com.example.FilterActivity protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/A.kt:53:30 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.NormalPermActivity. You could either make the component com.example.NormalPermActivity protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/A.kt:59:20 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.ExportedService. You could either make the component com.example.ExportedService protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/A.kt:66:46 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.ExportedReceiver. You could either make the component com.example.ExportedReceiver protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/JavaActivity.java:10:26 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.JavaActivity. You could either make the component com.example.JavaActivity protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/JavaActivity.java:16:23 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.JavaActivity. You could either make the component com.example.JavaActivity protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
	}
	assertFindings(t, manifestSourceFindings(t, "UnsafeIntentLaunch", manifest, sources), want)
}

func TestUnsafeIntentLaunchLintParityFlows(t *testing.T) {
	manifest := `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example">
    <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34" />
    <application>
        <activity android:name=".Lambdas" android:exported="true" />
        <activity android:name=".ViaBase" android:exported="true" />
        <activity android:name=".FragHost" android:exported="true" />
        <receiver android:name=".WhenReceiver" android:exported="true" />
        <receiver android:name=".JavaReceiver" android:exported="true" />
        <activity android:name="com.example.Sanitized" android:exported="true" />
    </application>
</manifest>
`
	sources := map[string]string{
		"com/example/B.kt": `package com.example

import android.app.Activity
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.os.Bundle
import androidx.fragment.app.Fragment

class Lambdas : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        intent.getParcelableExtra<Intent>("a")?.let { startActivity(it) }
        intent.getParcelableExtra<Intent>("b")?.let { next -> startActivity(next) }
        val c = intent.getParcelableExtra<Intent>("c")?.apply { putExtra("k", 1) }
        startActivity(c)
        val d = unwrap(intent)
        startActivity(d)
        val e = intent.getParcelableExtra<Intent>("e")
        startActivity(Intent(e))
        val f = intent.extras?.getParcelable<Intent>("f")
        startActivity(f)
        val g = intent.getParcelableExtra<Intent>("g")
        val copy = g
        startActivity(copy)
    }
    private fun unwrap(i: Intent): Intent? = i.getParcelableExtra("inner")
}

open class BaseActivity : Activity()

class ViaBase : BaseActivity() {
    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        startActivityForResult(intent, 1)
    }
}

class Frag : Fragment() {
    fun go() {
        val i = requireActivity().intent.getParcelableExtra<Intent>("x")
        startActivity(i)
        requireActivity().startActivity(requireActivity().intent)
    }
}

class WhenReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        when (intent.action) {
            Intent.ACTION_BOOT_COMPLETED -> context.startActivity(intent)
            "com.example.CUSTOM" -> context.startService(intent)
        }
        val action = intent.action
        if (Intent.ACTION_SCREEN_ON.equals(action)) {
            context.sendBroadcast(intent)
        }
    }
}

class Sanitized : Activity() {
    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        val safe = androidx.core.content.IntentSanitizer.Builder().build().sanitizeByFiltering(intent)
        startActivity(safe)
    }
}
`,
		"com/example/JavaReceiver.java": `package com.example;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;

public class JavaReceiver extends BroadcastReceiver {
    @Override public void onReceive(Context context, Intent intent) {
        switch (intent.getAction()) {
            case Intent.ACTION_BOOT_COMPLETED:
                context.startActivity(intent);
                break;
            default:
                context.startService(intent);
        }
    }
}
`,
	}
	want := []string{
		"com/example/B.kt:13:9 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.Lambdas. You could either make the component com.example.Lambdas protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/B.kt:14:9 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.Lambdas. You could either make the component com.example.Lambdas protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/B.kt:15:17 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.Lambdas. You could either make the component com.example.Lambdas protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/B.kt:21:17 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.Lambdas. You could either make the component com.example.Lambdas protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/B.kt:23:17 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.Lambdas. You could either make the component com.example.Lambdas protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/B.kt:33:30 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.ViaBase. You could either make the component com.example.ViaBase protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/B.kt:48:46 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.WhenReceiver. You could either make the component com.example.WhenReceiver protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
		"com/example/JavaReceiver.java:8:54 This intent could be coming from an untrusted source. It is later launched by an unprotected component com.example.JavaReceiver. You could either make the component com.example.JavaReceiver protected; or sanitize this intent using androidx.core.content.IntentSanitizer.",
	}
	assertFindings(t, manifestSourceFindings(t, "UnsafeIntentLaunch", manifest, sources), want)
}

func TestManifestSourceSecuritySuppressionsAndLookalikes(t *testing.T) {
	provider := `package com.example

import android.content.ContentProvider
import android.content.ContentValues
import android.database.Cursor
import android.net.Uri

class NotesProvider : ContentProvider() {
    override fun onCreate() = true
    override fun query(uri: Uri, p: Array<String>?, s: String?, a: Array<String>?, o: String?): Cursor? = null
    override fun getType(uri: Uri): String? = null
    override fun insert(uri: Uri, values: ContentValues?): Uri? = uri
    override fun update(uri: Uri, values: ContentValues?, s: String?, a: Array<String>?): Int = 0
    override fun delete(uri: Uri, s: String?, a: Array<String>?): Int = 0
}
`
	ignored := `<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:tools="http://schemas.android.com/tools" package="com.example">
    <application>
        <provider android:name=".NotesProvider" android:authorities="a" android:readPermission="com.example.READ"
            tools:ignore="ProviderReadPermissionOnly" />
    </application>
</manifest>
`
	if got := manifestSourceFindings(t, "ProviderReadPermissionOnly", ignored, map[string]string{"com/example/NotesProvider.kt": provider}); len(got) != 0 {
		t.Errorf("tools:ignore did not silence ProviderReadPermissionOnly: %v", got)
	}
	notProvider := strings.Replace(provider, "ContentProvider()", "LookalikeProvider()", 1) + "\nabstract class LookalikeProvider\n"
	plain := strings.Replace(ignored, `tools:ignore="ProviderReadPermissionOnly"`, "", 1)
	if got := manifestSourceFindings(t, "ProviderReadPermissionOnly", plain, map[string]string{"com/example/NotesProvider.kt": notProvider}); len(got) != 0 {
		t.Errorf("reported a class that does not extend ContentProvider: %v", got)
	}

	launchManifest := `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example">
    <application>
        <activity android:name=".Trampoline" android:exported="true" />
    </application>
</manifest>
`
	suppressed := `package com.example

import android.annotation.SuppressLint
import android.app.Activity
import android.content.Intent
import android.os.Bundle

class Trampoline : Activity() {
    @SuppressLint("UnsafeIntentLaunch")
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        startActivity(intent.getParcelableExtra<Intent>("next"))
    }
}
`
	if got := manifestSourceFindings(t, "UnsafeIntentLaunch", launchManifest, map[string]string{"com/example/Trampoline.kt": suppressed}); len(got) != 0 {
		t.Errorf("@SuppressLint did not silence UnsafeIntentLaunch: %v", got)
	}
	fragmentLookalike := `package com.example

import android.content.Intent

open class Screen {
    fun startActivity(intent: Intent?) {}
    val intent: Intent = Intent()
}

class Trampoline : Screen() {
    fun go() {
        startActivity(intent.getParcelableExtra<Intent>("next"))
    }
}
`
	if got := manifestSourceFindings(t, "UnsafeIntentLaunch", launchManifest, map[string]string{"com/example/Trampoline.kt": fragmentLookalike}); len(got) != 0 {
		t.Errorf("reported a non-Activity class with lookalike intent and startActivity members: %v", got)
	}

	implicitManifest := `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example">
    <application>
        <activity android:name=".Internal" android:exported="false">
            <intent-filter><action android:name="com.example.INTERNAL" /></intent-filter>
        </activity>
    </application>
</manifest>
`
	localIntent := `package com.example

import android.content.Context

class Intent(val action: String)

class Sender(private val context: Context) {
    fun send(start: (Intent) -> Unit) {
        start(Intent("com.example.INTERNAL"))
    }
}
`
	if got := manifestSourceFindings(t, "UnsafeImplicitIntentLaunch", implicitManifest, map[string]string{"com/example/Sender.kt": localIntent}); len(got) != 0 {
		t.Errorf("reported a local Intent lookalike: %v", got)
	}
}

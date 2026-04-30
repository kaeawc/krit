package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func runDatabaseQueryOnMainThread(t *testing.T, snippets ...string) []scanner.Finding {
	t.Helper()
	var files []*scanner.File
	for _, code := range snippets {
		files = append(files, parseInline(t, code))
	}
	return runDatabaseQueryOnMainThreadFiles(t, files...)
}

func parseJavaInline(t *testing.T, code string) *scanner.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Test.java")
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseJavaFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func runDatabaseQueryOnMainThreadFiles(t *testing.T, files ...*scanner.File) []scanner.Finding {
	t.Helper()
	return runDatabaseQueryOnMainThreadFilesWithFacts(t, nil, files...)
}

func runRuleByNameOnJavaWithResolver(t *testing.T, ruleName string, code string) []scanner.Finding {
	t.Helper()
	file := parseJavaInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	for _, r := range v2rules.Registry {
		if r.ID == ruleName {
			dispatcher := rules.NewDispatcherV2([]*v2rules.Rule{r}, resolver)
			cols := dispatcher.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %q not found in registry", ruleName)
	return nil
}

func runDatabaseQueryOnMainThreadFilesWithFacts(t *testing.T, facts *librarymodel.Facts, files ...*scanner.File) []scanner.Finding {
	t.Helper()
	for _, r := range v2rules.Registry {
		if r.ID != "DatabaseQueryOnMainThread" {
			continue
		}
		ctx := &v2rules.Context{
			ParsedFiles:  files,
			Collector:    scanner.NewFindingCollector(0),
			Rule:         r,
			LibraryFacts: facts,
		}
		r.Check(ctx)
		return ctx.Collector.Columns().Findings()
	}
	t.Fatal("DatabaseQueryOnMainThread rule not found")
	return nil
}

func runParsedFilesRule(t *testing.T, ruleName string, files ...*scanner.File) []scanner.Finding {
	t.Helper()
	return runParsedFilesRuleWithFacts(t, ruleName, nil, files...)
}

func runParsedFilesRuleWithFacts(t *testing.T, ruleName string, facts *librarymodel.Facts, files ...*scanner.File) []scanner.Finding {
	t.Helper()
	for _, r := range v2rules.Registry {
		if r.ID != ruleName {
			continue
		}
		ctx := &v2rules.Context{
			ParsedFiles:  files,
			Collector:    scanner.NewFindingCollector(0),
			Rule:         r,
			LibraryFacts: facts,
		}
		r.Check(ctx)
		return ctx.Collector.Columns().Findings()
	}
	t.Fatalf("%s rule not found", ruleName)
	return nil
}

func TestDatabaseQueryOnMainThread_Fixtures(t *testing.T) {
	root := fixtureRoot(t)

	t.Run("positive fixture includes SQLite Room and SQLDelight", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "positive", "resource-cost", "DatabaseQueryOnMainThread.kt"))
		if err != nil {
			t.Fatal(err)
		}
		findings := runDatabaseQueryOnMainThreadFiles(t, file)
		if len(findings) != 4 {
			t.Fatalf("expected 4 findings for positive fixture, got %d", len(findings))
		}
	})

	t.Run("negative fixture keeps repository wrappers clean", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "negative", "resource-cost", "DatabaseQueryOnMainThread.kt"))
		if err != nil {
			t.Fatal(err)
		}
		findings := runDatabaseQueryOnMainThreadFiles(t, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for negative fixture, got %d", len(findings))
		}
	})
}

func TestRoomLoadsAllWhereFirstUsed_Fixtures(t *testing.T) {
	root := fixtureRoot(t)

	t.Run("positive fixture requires Room DAO evidence", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "positive", "resource-cost", "RoomLoadsAllWhereFirstUsed.kt"))
		if err != nil {
			t.Fatal(err)
		}
		findings := runParsedFilesRule(t, "RoomLoadsAllWhereFirstUsed", file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding for positive fixture, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "Room DAO getAll().first()") ||
			!strings.Contains(findings[0].Message, "LIMIT 1") {
			t.Fatalf("expected explicit Room DAO and LIMIT guidance, got %q", findings[0].Message)
		}
	})

	t.Run("negative fixture ignores limited Room queries and plain collections", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "negative", "resource-cost", "RoomLoadsAllWhereFirstUsed.kt"))
		if err != nil {
			t.Fatal(err)
		}
		findings := runParsedFilesRule(t, "RoomLoadsAllWhereFirstUsed", file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for negative fixture, got %d", len(findings))
		}
	})
}

func TestBufferedReadWithoutBuffer_Java(t *testing.T) {
	t.Run("positive FileInputStream read", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "BufferedReadWithoutBuffer", `
package test;
import java.io.FileInputStream;
import java.io.IOException;
class Reader {
  int read(String path, byte[] bytes) throws IOException {
    FileInputStream input = new FileInputStream(path);
    return input.read(bytes);
  }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected Java FileInputStream read finding, got %d", len(findings))
		}
	})
	t.Run("negative BufferedInputStream wrapper", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "BufferedReadWithoutBuffer", `
package test;
import java.io.BufferedInputStream;
import java.io.FileInputStream;
import java.io.IOException;
class Reader {
  int read(String path, byte[] bytes) throws IOException {
    BufferedInputStream input = new BufferedInputStream(new FileInputStream(path));
    return input.read(bytes);
  }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for BufferedInputStream wrapper, got %d", len(findings))
		}
	})
	t.Run("negative local lookalike", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "BufferedReadWithoutBuffer", `
package test;
class FileInputStream {
  int read(byte[] bytes) { return 0; }
}
class Reader {
  int read(byte[] bytes) {
    FileInputStream input = new FileInputStream();
    return input.read(bytes);
  }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for local lookalike, got %d", len(findings))
		}
	})
}

func TestCursorLoopWithColumnIndexInLoop_Java(t *testing.T) {
	t.Run("positive getColumnIndex inside moveToNext loop", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "CursorLoopWithColumnIndexInLoop", `
package test;
import android.database.Cursor;
class Reader {
  void read(Cursor cursor) {
    while (cursor.moveToNext()) {
      int index = cursor.getColumnIndex("name");
    }
  }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected Java cursor-loop finding, got %d", len(findings))
		}
	})
	t.Run("negative hoisted getColumnIndex", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "CursorLoopWithColumnIndexInLoop", `
package test;
import android.database.Cursor;
class Reader {
  void read(Cursor cursor) {
    int index = cursor.getColumnIndex("name");
    while (cursor.moveToNext()) {
      cursor.getString(index);
    }
  }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for hoisted column lookup, got %d", len(findings))
		}
	})
}

func TestRecyclerAdapterResourceRules_Java(t *testing.T) {
	t.Run("without DiffUtil positive", func(t *testing.T) {
		findings := runRuleByNameOnJavaWithResolver(t, "RecyclerAdapterWithoutDiffUtil", `
package test;
import androidx.recyclerview.widget.RecyclerView;
class MyAdapter extends RecyclerView.Adapter<RecyclerView.ViewHolder> {
  void refresh() {
    notifyDataSetChanged();
  }
}
}`)
		if len(findings) != 1 {
			t.Fatalf("expected Java RecyclerAdapterWithoutDiffUtil finding, got %d", len(findings))
		}
	})
	t.Run("without DiffUtil negative ListAdapter", func(t *testing.T) {
		findings := runRuleByNameOnJavaWithResolver(t, "RecyclerAdapterWithoutDiffUtil", `
package test;
import androidx.recyclerview.widget.ListAdapter;
import androidx.recyclerview.widget.RecyclerView;
class MyAdapter extends ListAdapter<String, RecyclerView.ViewHolder> {
  void refresh() {
    notifyDataSetChanged();
  }
}
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for ListAdapter, got %d", len(findings))
		}
	})
	t.Run("stable IDs positive", func(t *testing.T) {
		findings := runRuleByNameOnJavaWithResolver(t, "RecyclerAdapterStableIdsDefault", `
package test;
import androidx.recyclerview.widget.RecyclerView;
class MyAdapter extends RecyclerView.Adapter<RecyclerView.ViewHolder> {
}
}`)
		if len(findings) != 1 {
			t.Fatalf("expected Java RecyclerAdapterStableIdsDefault finding, got %d", len(findings))
		}
	})
	t.Run("stable IDs negative enabled", func(t *testing.T) {
		findings := runRuleByNameOnJavaWithResolver(t, "RecyclerAdapterStableIdsDefault", `
package test;
import androidx.recyclerview.widget.RecyclerView;
class MyAdapter extends RecyclerView.Adapter<RecyclerView.ViewHolder> {
  MyAdapter() {
    setHasStableIds(true);
  }
}
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings when stable IDs are enabled, got %d", len(findings))
		}
	})
}

func TestDatabaseLibraryFacts_DisableRoomWhenKnownAbsent(t *testing.T) {
	file := parseInline(t, `
package test

annotation class Dao
annotation class Query(val value: String)

@Dao
interface UserDao {
  @Query("SELECT * FROM users")
  fun getAllUsers(): List<String>
}

class UserActivity(private val userDao: UserDao) : android.app.Activity() {
  override fun onCreate() {
    userDao.getAllUsers()
  }
}
`)
	facts := librarymodel.FactsForProfile(librarymodel.ProjectProfile{
		HasGradle:                    true,
		DependencyExtractionComplete: true,
		Dependencies: []librarymodel.Dependency{
			{Group: "com.squareup.okhttp3", Name: "okhttp", Version: "4.12.0"},
		},
	})
	findings := runDatabaseQueryOnMainThreadFilesWithFacts(t, facts, file)
	if len(findings) != 0 {
		t.Fatalf("expected no Room finding when project profile proves Room absent, got %d", len(findings))
	}
}

func TestDatabaseLibraryFacts_AllowFullyQualifiedRoomEvidenceWhenProfilePartial(t *testing.T) {
	file := parseInline(t, `
package test

@androidx.room.Dao
interface UserDao {
  @androidx.room.Query("SELECT * FROM users")
  fun getAllUsers(): List<String>
}

class UserActivity(private val userDao: UserDao) : android.app.Activity() {
  override fun onCreate() {
    userDao.getAllUsers()
  }
}
`)
	facts := librarymodel.FactsForProfile(librarymodel.ProjectProfile{
		HasGradle:                   true,
		HasUnresolvedDependencyRefs: true,
	})
	findings := runDatabaseQueryOnMainThreadFilesWithFacts(t, facts, file)
	if len(findings) != 1 {
		t.Fatalf("expected Room finding from fully-qualified source evidence with partial profile, got %d", len(findings))
	}
}

func TestDatabaseLibraryFacts_DoNotAllowLocalRoomLookalikeWhenKnownAbsent(t *testing.T) {
	file := parseInline(t, `
package test

annotation class Dao
annotation class Query(val value: String)

@Dao
interface UserDao {
  @Query("SELECT * FROM users")
  fun getAllUsers(): List<String>
}

class UserActivity(private val userDao: UserDao) : android.app.Activity() {
  override fun onCreate() {
    userDao.getAllUsers()
  }
}
`)
	facts := librarymodel.FactsForProfile(librarymodel.ProjectProfile{
		HasGradle:                    true,
		DependencyExtractionComplete: true,
	})
	findings := runDatabaseQueryOnMainThreadFilesWithFacts(t, facts, file)
	if len(findings) != 0 {
		t.Fatalf("expected no local Room-lookalike finding when profile proves Room absent, got %d", len(findings))
	}
}

func TestDatabaseLibraryFacts_EnableRoomWhenDependencyPresent(t *testing.T) {
	file := parseInline(t, `
package test

annotation class Dao
annotation class Query(val value: String)

@Dao
interface UserDao {
  @Query("SELECT * FROM users")
  fun getAllUsers(): List<String>
}

class UserActivity(private val userDao: UserDao) : android.app.Activity() {
  override fun onCreate() {
    userDao.getAllUsers()
  }
}
`)
	facts := librarymodel.FactsForProfile(librarymodel.ProjectProfile{
		HasGradle: true,
		Dependencies: []librarymodel.Dependency{
			{Group: "androidx.room", Name: "room-runtime", Version: "2.6.1"},
		},
	})
	findings := runDatabaseQueryOnMainThreadFilesWithFacts(t, facts, file)
	if len(findings) != 1 {
		t.Fatalf("expected Room finding when project profile includes Room, got %d", len(findings))
	}
}

func TestOkHttpCallExecuteSync_PositiveSuspendCall(t *testing.T) {
	findings := runRuleByName(t, "OkHttpCallExecuteSync", `
package test

class NetworkRepository {
  suspend fun fetchData(call: okhttp3.Call): String {
    val response = call.execute()
    return response.body?.string() ?: ""
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected finding for OkHttp Call.execute in suspend function, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "OkHttp Call.execute()") ||
		!strings.Contains(findings[0].Message, "withContext(Dispatchers.IO)") {
		t.Fatalf("expected explicit OkHttp and IO dispatcher guidance, got %q", findings[0].Message)
	}
}

func TestOkHttpCallExecuteSync_NegativeCustomExecute(t *testing.T) {
	findings := runRuleByName(t, "OkHttpCallExecuteSync", `
package test

class Response {
  fun execute(): String = "ok"
}

class Repository {
  suspend fun run(response: Response): String {
    return response.execute()
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no finding for non-OkHttp execute() receiver, got %d", len(findings))
	}
}

func TestOkHttpCallExecuteSync_NegativeIODispatcher(t *testing.T) {
	findings := runRuleByName(t, "OkHttpCallExecuteSync", `
package test

class NetworkRepository {
  suspend fun fetchData(call: okhttp3.Call): String {
    return kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) {
      val response = call.execute()
      response.body?.string() ?: ""
    }
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no finding when OkHttp execute is already inside Dispatchers.IO, got %d", len(findings))
	}
}

func TestOkHttpClientCreatedPerCall_NegativeLocalTypeName(t *testing.T) {
	findings := runRuleByName(t, "OkHttpClientCreatedPerCall", `
package test

class OkHttpClient {
  class Builder {
    fun build(): OkHttpClient = OkHttpClient()
  }
}

class LocalFactory {
  fun create(): OkHttpClient {
    return OkHttpClient.Builder().build()
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no finding for local OkHttpClient type without okhttp3 import, got %d", len(findings))
	}
}

func TestOkHttpClientCreatedPerCall_PositiveJavaDirectConstruction(t *testing.T) {
	findings := runRuleByNameOnJava(t, "OkHttpClientCreatedPerCall", `
package test;

import okhttp3.OkHttpClient;

class Factory {
  OkHttpClient create() {
    return new OkHttpClient();
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected finding for Java OkHttpClient construction, got %d", len(findings))
	}
}

func TestOkHttpClientCreatedPerCall_PositiveJavaBuilder(t *testing.T) {
	findings := runRuleByNameOnJava(t, "OkHttpClientCreatedPerCall", `
package test;

import okhttp3.OkHttpClient;

class Factory {
  OkHttpClient create() {
    return new OkHttpClient.Builder().build();
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected finding for Java OkHttpClient.Builder().build(), got %d", len(findings))
	}
}

func TestOkHttpClientCreatedPerCall_NegativeJavaLocalTypeName(t *testing.T) {
	findings := runRuleByNameOnJava(t, "OkHttpClientCreatedPerCall", `
package test;

class OkHttpClient {
  static class Builder {
    OkHttpClient build() { return new OkHttpClient(); }
  }
}

class Factory {
  OkHttpClient create() {
    return new OkHttpClient.Builder().build();
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no finding for Java local OkHttpClient type without okhttp3 import, got %d", len(findings))
	}
}

func TestOkHttpClientCreatedPerCall_NegativeJavaStaticSingletonAssignment(t *testing.T) {
	findings := runRuleByNameOnJava(t, "OkHttpClientCreatedPerCall", `
package test;

import okhttp3.OkHttpClient;

class Factory {
  private static volatile OkHttpClient internalClient;

  static OkHttpClient getInternalClient() {
    if (internalClient == null) {
      synchronized (Factory.class) {
        if (internalClient == null) {
          internalClient = new OkHttpClient.Builder().build();
        }
      }
    }
    return internalClient;
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no finding for Java static singleton assignment, got %d", len(findings))
	}
}

func TestRetrofitCreateInHotPath_NegativeLocalTypeName(t *testing.T) {
	findings := runRuleByName(t, "RetrofitCreateInHotPath", `
package test

class Retrofit {
  class Builder {
    fun build(): Retrofit = Retrofit()
  }
  fun create(type: Class<*>): Any = Any()
}

class LocalFactory {
  fun create(): Any {
    return Retrofit.Builder().build().create(String::class.java)
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no finding for local Retrofit type without retrofit2 import, got %d", len(findings))
	}
}

func TestRetrofitCreateInHotPath_PositiveJavaBuilderChain(t *testing.T) {
	findings := runRuleByNameOnJava(t, "RetrofitCreateInHotPath", `
package test;

import retrofit2.Retrofit;

interface Api {}

class Factory {
  Api create() {
    return new Retrofit.Builder().baseUrl("https://example.com").build().create(Api.class);
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected finding for Java Retrofit builder hot path, got %d", len(findings))
	}
}

func TestRetrofitCreateInHotPath_NegativeJavaLocalTypeName(t *testing.T) {
	findings := runRuleByNameOnJava(t, "RetrofitCreateInHotPath", `
package test;

class Retrofit {
  static class Builder {
    Builder baseUrl(String url) { return this; }
    Retrofit build() { return new Retrofit(); }
  }
  <T> T create(Class<T> type) { return null; }
}

interface Api {}

class Factory {
  Api create() {
    return new Retrofit.Builder().baseUrl("https://example.com").build().create(Api.class);
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no finding for Java local Retrofit type without retrofit2 import, got %d", len(findings))
	}
}

func TestHttpClientNotReused_PositiveJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "HttpClientNotReused", `
package test;

import java.net.http.HttpClient;

class Factory {
  HttpClient create() {
    return HttpClient.newHttpClient();
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected finding for Java HttpClient.newHttpClient(), got %d", len(findings))
	}
}

func TestHttpClientNotReused_NegativeJavaLocalTypeName(t *testing.T) {
	findings := runRuleByNameOnJava(t, "HttpClientNotReused", `
package test;

class HttpClient {
  static HttpClient newHttpClient() { return new HttpClient(); }
}

class Factory {
  HttpClient create() {
    return HttpClient.newHttpClient();
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no finding for Java local HttpClient type without java.net.http import, got %d", len(findings))
	}
}

func TestDatabaseInstanceRecreated_NegativeLocalRoomTypeName(t *testing.T) {
	findings := runRuleByName(t, "DatabaseInstanceRecreated", `
package test

object Room {
  fun databaseBuilder(context: Context, klass: Class<AppDb>, name: String): Builder = Builder()
}
class Context
class Builder {
  fun build(): AppDb = AppDb()
}
class AppDb

class Repository(private val context: Context) {
  fun load(): AppDb {
    return Room.databaseBuilder(context, AppDb::class.java, "app.db").build()
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no finding for local Room type without androidx.room import, got %d", len(findings))
	}
}

func TestDatabaseQueryOnMainThread_PositiveActivityLifecycle(t *testing.T) {
	findings := runDatabaseQueryOnMainThread(t, `
package test

class MainActivity : android.app.Activity() {
  override fun onCreate(savedInstanceState: android.os.Bundle?) {
    val db: android.database.sqlite.SQLiteDatabase = TODO()
    db.rawQuery("SELECT * FROM users", null)
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected finding for SQLite query in Activity.onCreate, got %d", len(findings))
	}
}

func TestDatabaseQueryOnMainThread_PositiveListenerLambda(t *testing.T) {
	findings := runDatabaseQueryOnMainThread(t, `
package test

class UserScreen {
  fun bind(button: android.view.View, db: android.database.sqlite.SQLiteDatabase) {
    button.setOnClickListener {
      db.query("users", null, null, null, null, null, null)
    }
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected finding for SQLite query in click listener, got %d", len(findings))
	}
}

func TestDatabaseQueryOnMainThread_PositiveMainThreadAnnotation(t *testing.T) {
	findings := runDatabaseQueryOnMainThread(t, `
package test

class UserRepository {
  @androidx.annotation.MainThread
  fun loadOnMain(db: android.database.sqlite.SQLiteDatabase) {
    db.execSQL("DELETE FROM users")
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected finding for SQLite query in @MainThread function, got %d", len(findings))
	}
}

func TestDatabaseQueryOnMainThread_PositiveSameClassHelper(t *testing.T) {
	findings := runDatabaseQueryOnMainThread(t, `
package test

class MainActivity : android.app.Activity() {
  override fun onCreate(savedInstanceState: android.os.Bundle?) {
    loadUsers()
  }

  private fun loadUsers() {
    val db: android.database.sqlite.SQLiteDatabase = TODO()
    db.rawQuery("SELECT * FROM users", null)
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected finding for Activity.onCreate calling DB helper, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "loadUsers()") ||
		!strings.Contains(findings[0].Message, "SQLiteDatabase.rawQuery()") ||
		!strings.Contains(findings[0].Message, "resume lifecycle/UI work on the main thread") {
		t.Fatalf("expected explicit helper, database target, and fix guidance, got %q", findings[0].Message)
	}
}

func TestDatabaseQueryOnMainThread_PositiveJavaSignalHelperMessage(t *testing.T) {
	file := parseJavaInline(t, `
package test;

class Cursor {}
class SignalSQLiteDatabase {
  Cursor query(String table, String[] columns, String selection, String[] args, String groupBy, String having, String orderBy) { return new Cursor(); }
}
class DatabaseTable {
  SignalSQLiteDatabase readableDatabase;
}
class ThreadTable extends DatabaseTable {
  long getOrCreateThreadIdFor(Object recipient) {
    readableDatabase.query("thread", null, "recipient_id = ?", null, null, null, null);
    return 1L;
  }
}
class SignalDatabase {
  static ThreadTable threads() { return new ThreadTable(); }
}
class SmsSendtoActivity extends android.app.Activity {
  protected void onCreate(android.os.Bundle savedInstanceState) {
    startActivity(getNextIntent(getIntent()));
  }
  private android.content.Intent getNextIntent(android.content.Intent original) {
    SignalDatabase.threads().getOrCreateThreadIdFor(new Object());
    return original;
  }
}
`)
	findings := runDatabaseQueryOnMainThreadFiles(t, file)
	if len(findings) != 1 {
		t.Fatalf("expected one Signal helper finding, got %d", len(findings))
	}
	message := findings[0].Message
	if !strings.Contains(message, "getNextIntent()") ||
		!strings.Contains(message, "SignalDatabase.threads().getOrCreateThreadIdFor") ||
		!strings.Contains(message, "Move the database work off the UI thread") ||
		!strings.Contains(message, "resume lifecycle/UI work on the main thread") {
		t.Fatalf("expected explicit Signal call and fix guidance, got %q", message)
	}
}

func TestDatabaseQueryOnMainThread_PositiveTransitiveSameClassHelper(t *testing.T) {
	findings := runDatabaseQueryOnMainThread(t, `
package test

class MainActivity : android.app.Activity() {
  override fun onCreate(savedInstanceState: android.os.Bundle?) {
    refresh()
  }

  private fun refresh() {
    loadUsers()
  }

  private fun loadUsers() {
    val db: android.database.sqlite.SQLiteDatabase = TODO()
    db.execSQL("DELETE FROM users")
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected finding for transitive DB helper call from Activity.onCreate, got %d", len(findings))
	}
}

func TestDatabaseQueryOnMainThread_PositiveListenerCallsHelper(t *testing.T) {
	findings := runDatabaseQueryOnMainThread(t, `
package test

class UserScreen {
  fun bind(button: android.view.View) {
    button.setOnClickListener {
      loadUsers()
    }
  }

  private fun loadUsers() {
    val db: android.database.sqlite.SQLiteDatabase = TODO()
    db.query("users", null, null, null, null, null, null)
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected finding for click listener calling DB helper, got %d", len(findings))
	}
}

func TestDatabaseQueryOnMainThread_NegativeDatabaseTableWrapper(t *testing.T) {
	findings := runDatabaseQueryOnMainThread(t, `
package test

class GroupTable {
  fun getGroup(readableDatabase: android.database.sqlite.SQLiteDatabase) {
    readableDatabase
      .query("groups", null, null, null, null, null, null)
      .close()
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for plain database table wrapper, got %d", len(findings))
	}
}

func TestDatabaseQueryOnMainThread_NegativeSQLiteOpenHelperLifecycle(t *testing.T) {
	findings := runDatabaseQueryOnMainThread(t, `
package test

class JobDatabase : android.database.sqlite.SQLiteOpenHelper(null, "jobs", null, 1) {
  override fun onCreate(db: android.database.sqlite.SQLiteDatabase) {
    db.execSQL("CREATE TABLE jobs (_id INTEGER)")
  }

  override fun onUpgrade(db: android.database.sqlite.SQLiteDatabase, oldVersion: Int, newVersion: Int) {
    db.execSQL("ALTER TABLE jobs ADD COLUMN priority INTEGER")
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for SQLiteOpenHelper lifecycle methods, got %d", len(findings))
	}
}

func TestDatabaseQueryOnMainThread_NegativeBackgroundDispatcher(t *testing.T) {
	findings := runDatabaseQueryOnMainThread(t, `
package test

class MainActivity : android.app.Activity() {
  fun bind(button: android.view.View, db: android.database.sqlite.SQLiteDatabase) {
    button.setOnClickListener {
      kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) {
        db.rawQuery("SELECT * FROM users", null)
      }
    }
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings inside Dispatchers.IO block, got %d", len(findings))
	}
}

func TestDatabaseQueryOnMainThread_NegativeJavaSimpleTaskSignalDatabase(t *testing.T) {
	file := parseJavaInline(t, `
package test;

class Cursor {}
class SignalSQLiteDatabase {
  Cursor query(String table, String[] columns, String selection, String[] args, String groupBy, String having, String orderBy) { return new Cursor(); }
}
class DatabaseTable {
  SignalSQLiteDatabase readableDatabase;
}
class MessageTable extends DatabaseTable {
  Cursor getMessageRecord(long id) {
    return readableDatabase.query("message", null, "_id = ?", null, null, null, null);
  }
}
class SignalDatabase {
  static MessageTable messages() { return new MessageTable(); }
}
class SimpleTask {
  static <T> void run(java.util.concurrent.Callable<T> work, java.util.function.Consumer<T> done) {}
}
class MediaPreviewFragment extends android.app.Fragment {
  public void onResume() {
    SimpleTask.run(() -> SignalDatabase.messages().getMessageRecord(1), result -> {});
  }
}
`)
	findings := runDatabaseQueryOnMainThreadFiles(t, file)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for SignalDatabase work inside Java SimpleTask.run, got %d", len(findings))
	}
}

func TestDatabaseQueryOnMainThread_NegativeJavaRxSubscribeOnSignalDatabase(t *testing.T) {
	file := parseJavaInline(t, `
package test;

class Cursor {}
class SignalSQLiteDatabase {
  Cursor query(String table, String[] columns, String selection, String[] args, String groupBy, String having, String orderBy) { return new Cursor(); }
}
class DatabaseTable {
  SignalSQLiteDatabase readableDatabase;
}
class GroupTable extends DatabaseTable {
  Cursor getGroup(long id) {
    return readableDatabase.query("groups", null, "_id = ?", null, null, null, null);
  }
}
class SignalDatabase {
  static GroupTable groups() { return new GroupTable(); }
}
class Single<T> {
  static <T> Single<T> fromCallable(java.util.concurrent.Callable<T> callable) { return new Single<T>(); }
  Single<T> subscribeOn(Object scheduler) { return this; }
  void subscribe(java.util.function.Consumer<T> consumer) {}
}
class Schedulers {
  static Object io() { return new Object(); }
}
class ShowAdminsBottomSheetDialog extends android.app.Fragment {
  public void onViewCreated(android.view.View view, android.os.Bundle savedInstanceState) {
    Single.fromCallable(() -> getAdmins(1)).subscribeOn(Schedulers.io()).subscribe(admins -> {});
  }
  private static Cursor getAdmins(long groupId) {
    return SignalDatabase.groups().getGroup(groupId);
  }
}
`)
	findings := runDatabaseQueryOnMainThreadFiles(t, file)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for SignalDatabase work inside Java Rx subscribeOn(IO), got %d", len(findings))
	}
}

func TestDatabaseQueryOnMainThread_NegativeLifecycleNameWithoutLifecycleOwner(t *testing.T) {
	findings := runDatabaseQueryOnMainThread(t, `
package test

class FakeActivityNameOnly {
  fun onCreate(savedInstanceState: android.os.Bundle?) {
    val db: android.database.sqlite.SQLiteDatabase = TODO()
    db.rawQuery("SELECT * FROM users", null)
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no finding when lifecycle-like method is not on a lifecycle owner, got %d", len(findings))
	}
}

func TestDatabaseQueryOnMainThread_NegativeJavaLifecycleNameWithoutLifecycleOwner(t *testing.T) {
	file := parseJavaInline(t, `
package test;

class FakeActivityNameOnly {
  protected void onCreate(android.os.Bundle savedInstanceState) {
    android.database.sqlite.SQLiteDatabase db = null;
    db.rawQuery("SELECT * FROM users", null);
  }
}
`)
	findings := runDatabaseQueryOnMainThreadFiles(t, file)
	if len(findings) != 0 {
		t.Fatalf("expected no finding when Java lifecycle-like method is not on a lifecycle owner, got %d", len(findings))
	}
}

func TestDatabaseQueryOnMainThread_PositiveDeferredUiCallbackDoesNotPropagateToBinder(t *testing.T) {
	findings := runDatabaseQueryOnMainThread(t, `
package test

open class DatabaseTable {
  val readableDatabase: SignalSQLiteDatabase = TODO()
}
class SignalSQLiteDatabase {
  fun query(table: String, columns: Array<String>?, selection: String?, selectionArgs: Array<String>?, groupBy: String?, having: String?, orderBy: String?): Cursor = TODO()
}
class Cursor
class MessageTable : DatabaseTable() {
  fun getMessageRecord(id: Long): Cursor {
    return readableDatabase.query("message", null, "_id = ?", arrayOf(id.toString()), null, null, null)
  }
}
object SignalDatabase {
  val messages: MessageTable = MessageTable()
}
class Toolbar {
  fun setOnMenuItemClickListener(listener: () -> Boolean) = Unit
}
class MediaFragment : android.app.Fragment() {
  override fun onCreate(savedInstanceState: android.os.Bundle?) {
    bindMenuItems(Toolbar())
  }
  private fun bindMenuItems(toolbar: Toolbar) {
    toolbar.setOnMenuItemClickListener {
      deleteMedia()
      true
    }
  }
  private fun deleteMedia() {
    SignalDatabase.messages.getMessageRecord(1)
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding at the UI callback helper call, got %d", len(findings))
	}
}

func TestDatabaseQueryOnMainThread_NegativeHelperBehindIODispatcher(t *testing.T) {
	findings := runDatabaseQueryOnMainThread(t, `
package test

class MainActivity : android.app.Activity() {
  override fun onCreate(savedInstanceState: android.os.Bundle?) {
    kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) {
      loadUsers()
    }
  }

  private fun loadUsers() {
    val db: android.database.sqlite.SQLiteDatabase = TODO()
    db.rawQuery("SELECT * FROM users", null)
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when helper call is behind Dispatchers.IO, got %d", len(findings))
	}
}

func TestDatabaseQueryOnMainThread_NegativeAmbiguousSameClassOverload(t *testing.T) {
	findings := runDatabaseQueryOnMainThread(t, `
package test

class MainActivity : android.app.Activity() {
  override fun onCreate(savedInstanceState: android.os.Bundle?) {
    loadUsers(42)
  }

  private fun loadUsers(id: Int) {
  }

  private fun loadUsers() {
    val db: android.database.sqlite.SQLiteDatabase = TODO()
    db.rawQuery("SELECT * FROM users", null)
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for ambiguous overloaded helper name, got %d", len(findings))
	}
}

func TestDatabaseQueryOnMainThread_NegativeNonDatabaseQueryMethod(t *testing.T) {
	findings := runDatabaseQueryOnMainThread(t, `
package test

class MainActivity : android.app.Activity() {
  override fun onCreate(savedInstanceState: android.os.Bundle?) {
    searchClient.query("users")
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for non-database query method, got %d", len(findings))
	}
}

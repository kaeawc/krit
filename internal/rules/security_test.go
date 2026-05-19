package rules_test

import (
	"strings"
	"testing"
)

func TestContentProviderQueryWithSelectionInterpolation_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "ContentProviderQueryWithSelectionInterpolation", `
package test

import android.content.ContentResolver
import android.net.Uri

class UserLookup {
    fun load(resolver: ContentResolver, uri: Uri, name: String) {
        resolver.query(uri, null, "name = '$name'", null, null)
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "selectionArgs") {
		t.Fatalf("expected selectionArgs guidance, got %q", findings[0].Message)
	}
}

func TestContentProviderQueryWithSelectionInterpolation_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "ContentProviderQueryWithSelectionInterpolation", `
package test

import android.content.ContentResolver
import android.net.Uri

class UserLookup {
    fun load(resolver: ContentResolver, uri: Uri, name: String) {
        resolver.query(uri, null, "name = ?", arrayOf(name), null)
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestContentProviderQueryWithSelectionInterpolation_IgnoresNonResolverQueries(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "ContentProviderQueryWithSelectionInterpolation", `
package test

class DatabaseLookup {
    fun load(db: Any, tableName: String, name: String) {
        db.query(tableName, null, "name = '$name'", null, null)
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestSqlInjectionRawQuery_KotlinInterpolatedRawQuery(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "SqlInjectionRawQuery", `
package test

import android.database.sqlite.SQLiteDatabase

class UserDao(private val db: SQLiteDatabase) {
    fun load(id: String) {
        db.rawQuery("SELECT * FROM users WHERE id = $id", null)
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "placeholders") {
		t.Fatalf("expected placeholder guidance, got %q", findings[0].Message)
	}
}

func TestSqlInjectionRawQuery_KotlinComputedExecSQL(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "SqlInjectionRawQuery", `
package test

import android.database.sqlite.SQLiteDatabase

class UserDao {
    fun delete(db: SQLiteDatabase, userId: String) {
        db.execSQL("DELETE FROM users WHERE id = " + userId)
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestSqlInjectionRawQuery_KotlinQuerySelection(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "SqlInjectionRawQuery", `
package test

import android.database.sqlite.SQLiteDatabase

class UserDao {
    fun load(db: SQLiteDatabase, userId: String) {
        db.query("users", null, "id = $userId", null, null, null, null)
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestSqlInjectionRawQuery_KotlinParameterizedAndSchemaConstantsAreClean(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "SqlInjectionRawQuery", `
package test

import android.database.sqlite.SQLiteDatabase

private const val USERS_TABLE = "users"
private const val COLUMN_ID = "id"

class UserDao(private val db: SQLiteDatabase) {
    fun load(id: String) {
        db.rawQuery("SELECT * FROM users WHERE id = ?", arrayOf(id))
        db.rawQuery("SELECT * FROM $USERS_TABLE WHERE $COLUMN_ID = ?", arrayOf(id))
        db.rawQuery("SELECT * FROM " + USERS_TABLE + " WHERE " + COLUMN_ID + " = ?", arrayOf(id))
        db.query(USERS_TABLE, null, "$COLUMN_ID = ?", arrayOf(id), null, null, null)
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestSqlInjectionRawQuery_KotlinRejectsLocalLookalike(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "SqlInjectionRawQuery", `
package test

class QueryRunner {
    fun rawQuery(sql: String, args: Array<String>?) {}
}

class UserDao(private val db: QueryRunner) {
    fun load(id: String) {
        db.rawQuery("SELECT * FROM users WHERE id = $id", null)
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for local lookalike, got %d", len(findings))
	}
}

func TestSqlInjectionRawQuery_JavaComputedRawQuery(t *testing.T) {
	findings := runRuleByNameOnJavaWithResolver(t, "SqlInjectionRawQuery", `
package test;

import android.database.sqlite.SQLiteDatabase;

class UserDao {
    void load(SQLiteDatabase db, String id) {
        db.rawQuery("SELECT * FROM users WHERE id = " + id, null);
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java finding, got %d", len(findings))
	}
}

func TestSqlInjectionRawQuery_JavaParameterizedAndLookalikeClean(t *testing.T) {
	findings := runRuleByNameOnJavaWithResolver(t, "SqlInjectionRawQuery", `
package test;

import android.database.sqlite.SQLiteDatabase;

class QueryRunner {
    void rawQuery(String sql, String[] args) {}
}

class UserDao {
    static final String USERS_TABLE = "users";
    void load(SQLiteDatabase db, String id, QueryRunner runner) {
        db.rawQuery("SELECT * FROM users WHERE id = ?", new String[] { id });
        db.rawQuery("SELECT * FROM " + USERS_TABLE + " WHERE id = ?", new String[] { id });
        runner.rawQuery("SELECT * FROM users WHERE id = " + id, null);
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 Java findings, got %d", len(findings))
	}
}

func TestRuntimeExecUnsafeShape_KotlinInterpolated(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "RuntimeExecUnsafeShape", `
package test

class Runner {
    fun list(userPath: String) {
        Runtime.getRuntime().exec("ls -la $userPath")
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "String array") {
		t.Fatalf("expected array guidance, got %q", findings[0].Message)
	}
}

func TestRuntimeExecUnsafeShape_KotlinComputed(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "RuntimeExecUnsafeShape", `
package test

class Runner {
    fun list(userPath: String) {
        Runtime.getRuntime().exec("ls -la " + userPath)
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestRuntimeExecUnsafeShape_KotlinSafeOverloads(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "RuntimeExecUnsafeShape", `
package test

class Runner {
    fun list(userPath: String) {
        Runtime.getRuntime().exec(arrayOf("ls", "-la", userPath))
        Runtime.getRuntime().exec("ls -la")
        ProcessBuilder("ls", "-la", userPath).start()
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestRuntimeExecUnsafeShape_KotlinRejectsLocalRuntime(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "RuntimeExecUnsafeShape", `
package test

class Runtime {
    fun exec(command: String) {}
    companion object {
        fun getRuntime(): Runtime = Runtime()
    }
}

class Runner {
    fun list(userPath: String) {
        Runtime.getRuntime().exec("ls -la $userPath")
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for local Runtime lookalike, got %d", len(findings))
	}
}

func TestRuntimeExecUnsafeShape_JavaComputed(t *testing.T) {
	findings := runRuleByNameOnJavaWithResolver(t, "RuntimeExecUnsafeShape", `
package test;

class Runner {
    void list(String userPath) throws java.io.IOException {
        Runtime.getRuntime().exec("ls -la " + userPath);
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java finding, got %d", len(findings))
	}
}

func TestRuntimeExecUnsafeShape_JavaSafeAndLookalike(t *testing.T) {
	findings := runRuleByNameOnJavaWithResolver(t, "RuntimeExecUnsafeShape", `
package test;

class Runner {
    void list(String userPath) throws java.io.IOException {
        Runtime.getRuntime().exec(new String[] {"ls", "-la", userPath});
        Runtime.getRuntime().exec("ls -la");
        new ProcessBuilder("ls", "-la", userPath).start();
    }
}

class Runtime {
    static Runtime getRuntime() { return new Runtime(); }
    void exec(String command) {}
}

class LocalRunner {
    void run(String value) {
        Runtime.getRuntime().exec("local " + value);
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 Java findings, got %d", len(findings))
	}
}

func TestRoomRawQueryStringConcat_Interpolated(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "RoomRawQueryStringConcat", `
package test

import androidx.sqlite.db.SimpleSQLiteQuery

class UserDao {
    fun query(term: String) {
        SimpleSQLiteQuery("SELECT * FROM users WHERE name LIKE '%$term%'")
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "bindArgs") {
		t.Fatalf("expected bindArgs guidance, got %q", findings[0].Message)
	}
}

func TestRoomRawQueryStringConcat_Computed(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "RoomRawQueryStringConcat", `
package test

import androidx.sqlite.db.SimpleSQLiteQuery

class UserDao {
    fun query(term: String) {
        SimpleSQLiteQuery("SELECT * FROM users WHERE name LIKE '%" + term + "%'")
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestRoomRawQueryStringConcat_Negatives(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "RoomRawQueryStringConcat", `
package test

import androidx.sqlite.db.SimpleSQLiteQuery

private const val USERS_TABLE = "users"
private const val COLUMN_NAME = "name"

class UserDao {
    fun query(term: String) {
        SimpleSQLiteQuery("SELECT * FROM users WHERE name LIKE ?", arrayOf("%$term%"))
        SimpleSQLiteQuery("SELECT * FROM users")
        SimpleSQLiteQuery("SELECT * FROM $USERS_TABLE WHERE $COLUMN_NAME LIKE ?", arrayOf(term))
        SimpleSQLiteQuery("SELECT * FROM " + USERS_TABLE)
        ProcessBuilder("sqlite3", "db").start()
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestRoomRawQueryStringConcat_RequiresSimpleSQLiteQueryImport(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "RoomRawQueryStringConcat", `
package test

class SimpleSQLiteQuery(val query: String)

class UserDao {
    fun query(term: String) {
        SimpleSQLiteQuery("SELECT * FROM users WHERE name LIKE '%$term%'")
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for local SimpleSQLiteQuery lookalike, got %d", len(findings))
	}
}

func TestProcessBuilderShellArg_KotlinInterpolated(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "ProcessBuilderShellArg", `
package test

class Runner {
    fun grep(pattern: String) {
        ProcessBuilder("sh", "-c", "grep $pattern /var/log/app.log").start()
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "without a shell") {
		t.Fatalf("expected shell avoidance guidance, got %q", findings[0].Message)
	}
}

func TestProcessBuilderShellArg_KotlinListOfComputed(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "ProcessBuilderShellArg", `
package test

class Runner {
    fun grep(pattern: String) {
        ProcessBuilder(listOf("bash", "-c", "grep " + pattern + " /var/log/app.log")).start()
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestProcessBuilderShellArg_KotlinNegatives(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "ProcessBuilderShellArg", `
package test

class Runner {
    fun grep(pattern: String) {
        ProcessBuilder("grep", pattern, "/var/log/app.log").start()
        ProcessBuilder("sh", "-c", "grep fixed /var/log/app.log").start()
        ProcessBuilder("sh", "-lc", "grep $pattern /var/log/app.log").start()
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestProcessBuilderShellArg_KotlinRejectsLocalLookalike(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "ProcessBuilderShellArg", `
package test

class ProcessBuilder(vararg args: String) {
    fun start() {}
}

class Runner {
    fun grep(pattern: String) {
        ProcessBuilder("sh", "-c", "grep $pattern /var/log/app.log").start()
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for local ProcessBuilder lookalike, got %d", len(findings))
	}
}

func TestProcessBuilderShellArg_JavaComputed(t *testing.T) {
	findings := runRuleByNameOnJavaWithResolver(t, "ProcessBuilderShellArg", `
package test;

class Runner {
    void grep(String pattern) throws java.io.IOException {
        new ProcessBuilder("sh", "-c", "grep " + pattern + " /var/log/app.log").start();
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java finding, got %d", len(findings))
	}
}

func TestProcessBuilderShellArg_JavaNegatives(t *testing.T) {
	findings := runRuleByNameOnJavaWithResolver(t, "ProcessBuilderShellArg", `
package test;

class Runner {
    void grep(String pattern) throws java.io.IOException {
        new ProcessBuilder("grep", pattern, "/var/log/app.log").start();
        new ProcessBuilder("sh", "-c", "grep fixed /var/log/app.log").start();
    }
}

class ProcessBuilder {
    ProcessBuilder(String... args) {}
    void start() {}
}

class LocalRunner {
    void run(String pattern) {
        new ProcessBuilder("sh", "-c", "grep " + pattern).start();
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 Java findings, got %d", len(findings))
	}
}

func TestLogPii_KotlinAndroidLogInterpolated(t *testing.T) {
	findings := runRuleByName(t, "LogPii", `
package test

import android.util.Log

class AuthLogger {
    fun send(password: String, userId: String) {
        Log.d("Auth", "sending password=$password for user=$userId")
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "sensitive") {
		t.Fatalf("expected sensitive guidance, got %q", findings[0].Message)
	}
}

func TestLogPii_KotlinTimberAndPrintln(t *testing.T) {
	findings := runRuleByName(t, "LogPii", `
package test

import timber.log.Timber

class AuthLogger {
    fun send(token: String, sessionId: String) {
        Timber.i("token=$token")
        println("session=$sessionId")
    }
}`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
}

func TestLogPii_KotlinNegatives(t *testing.T) {
	findings := runRuleByName(t, "LogPii", `
package test

import android.util.Log

class AuthLogger {
    fun send(userId: String, password: String) {
        Log.d("Auth", "sending user=$userId")
        Log.d("Auth", "password=<redacted>")
    }
}

class LocalLog {
    fun d(tag: String, message: String) {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestLogPii_JavaConcat(t *testing.T) {
	findings := runRuleByNameOnJava(t, "LogPii", `
package test;

import android.util.Log;

class AuthLogger {
    void send(String authHeader, String userId) {
        Log.d("Auth", "auth=" + authHeader + " user=" + userId);
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java finding, got %d", len(findings))
	}
}

func TestLogPii_JavaNegatives(t *testing.T) {
	findings := runRuleByNameOnJava(t, "LogPii", `
package test;

import android.util.Log;

class AuthLogger {
    void send(String userId, String token) {
        Log.d("Auth", "user=" + userId);
        Log.d("Auth", "token=<redacted>");
    }
}

class Log {
    static void d(String tag, String message) {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 Java findings, got %d", len(findings))
	}
}

func TestLogPii_PrintlnRequiresStdoutResolution(t *testing.T) {
	// Bare println in a file that shadows kotlin.io.println with a local
	// function must not be treated as a PII sink.
	shadowed := runRuleByName(t, "LogPii", `
package test

fun println(message: String) {}

class AuthLogger {
    fun emit(password: String) {
        println("password=$password")
    }
}`)
	if len(shadowed) != 0 {
		t.Fatalf("shadowed println: expected 0 findings, got %d", len(shadowed))
	}

	// Receiver-qualified println on an unrelated object is not stdout.
	receiverShadowed := runRuleByName(t, "LogPii", `
package test

class Sink {
    fun println(message: String) {}
}

class AuthLogger {
    fun emit(password: String) {
        val sink = Sink()
        sink.println("password=$password")
    }
}`)
	if len(receiverShadowed) != 0 {
		t.Fatalf("receiver println: expected 0 findings, got %d", len(receiverShadowed))
	}

	// System.out.println / System.err.println both count.
	systemOut := runRuleByName(t, "LogPii", `
package test

class AuthLogger {
    fun emit(password: String) {
        System.out.println("password=$password")
        System.err.println("password=$password")
    }
}`)
	if len(systemOut) != 2 {
		t.Fatalf("System.out/err println: expected 2 findings, got %d", len(systemOut))
	}
}

func TestLogPii_JavaPrintlnRequiresStdoutReceiver(t *testing.T) {
	// A Java method named println on an unrelated class must not be treated
	// as a stdout sink.
	receiver := runRuleByNameOnJava(t, "LogPii", `
package test;

class Sink {
    void println(String message) {}
}

class AuthLogger {
    void emit(String password) {
        Sink sink = new Sink();
        sink.println("password=" + password);
    }
}`)
	if len(receiver) != 0 {
		t.Fatalf("receiver println: expected 0 findings, got %d", len(receiver))
	}

	systemOut := runRuleByNameOnJava(t, "LogPii", `
package test;

class AuthLogger {
    void emit(String password) {
        System.out.println("password=" + password);
        System.err.println("password=" + password);
    }
}`)
	if len(systemOut) != 2 {
		t.Fatalf("System.out/err println: expected 2 findings, got %d", len(systemOut))
	}
}

func TestJdbcStatementExecute_KotlinInterpolated(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "JdbcStatementExecute", `
package test

import java.sql.Connection

class UserDao {
    fun load(connection: Connection, id: String) {
        connection.createStatement().executeQuery("SELECT * FROM users WHERE id = $id")
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "PreparedStatement") {
		t.Fatalf("expected PreparedStatement guidance, got %q", findings[0].Message)
	}
}

func TestJdbcStatementExecute_KotlinComputedLocal(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "JdbcStatementExecute", `
package test

import java.sql.Connection

class UserDao {
    fun delete(connection: Connection, id: String) {
        val stmt = connection.createStatement()
        stmt.executeUpdate("DELETE FROM users WHERE id = " + id)
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestJdbcStatementExecute_KotlinNegatives(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "JdbcStatementExecute", `
package test

import java.sql.Connection

private const val USERS_TABLE = "users"
private const val COLUMN_ID = "id"

class UserDao {
    fun load(connection: Connection, id: String) {
        connection.createStatement().executeQuery("SELECT * FROM users")
        connection.createStatement().executeQuery("SELECT * FROM $USERS_TABLE WHERE $COLUMN_ID = 1")
        val prepared = connection.prepareStatement("SELECT * FROM users WHERE id = ?")
        prepared.executeQuery()
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestJdbcStatementExecute_JavaComputed(t *testing.T) {
	findings := runRuleByNameOnJavaWithResolver(t, "JdbcStatementExecute", `
package test;

import java.sql.Connection;
import java.sql.Statement;

class UserDao {
    void load(Connection connection, String id) throws Exception {
        Statement stmt = connection.createStatement();
        stmt.executeQuery("SELECT * FROM users WHERE id = " + id);
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java finding, got %d", len(findings))
	}
}

func TestJdbcStatementExecute_JavaNegatives(t *testing.T) {
	findings := runRuleByNameOnJavaWithResolver(t, "JdbcStatementExecute", `
package test;

import java.sql.Connection;
import java.sql.PreparedStatement;

class UserDao {
    void load(Connection connection, String id) throws Exception {
        connection.createStatement().executeQuery("SELECT * FROM users");
        PreparedStatement stmt = connection.prepareStatement("SELECT * FROM users WHERE id = ?");
        stmt.executeQuery();
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 Java findings, got %d", len(findings))
	}
}

func TestXmlExternalEntity_KotlinPositive(t *testing.T) {
	findings := runRuleByName(t, "XmlExternalEntity", `
package test

import javax.xml.parsers.DocumentBuilderFactory

class XmlLoader {
    fun load(input: java.io.InputStream) {
        val factory = DocumentBuilderFactory.newInstance()
        factory.newDocumentBuilder().parse(input)
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestXmlExternalEntity_KotlinNegatives(t *testing.T) {
	findings := runRuleByName(t, "XmlExternalEntity", `
package test

import javax.xml.XMLConstants
import javax.xml.parsers.DocumentBuilderFactory
import javax.xml.stream.XMLInputFactory

class XmlLoader {
    fun load(input: java.io.InputStream) {
        val factory = DocumentBuilderFactory.newInstance()
        factory.setFeature("http://apache.org/xml/features/disallow-doctype-decl", true)
        factory.newDocumentBuilder().parse(input)

        val streamFactory = XMLInputFactory.newInstance()
        streamFactory.setProperty(XMLInputFactory.SUPPORT_DTD, false)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestXmlExternalEntity_JavaPositive(t *testing.T) {
	findings := runRuleByNameOnJava(t, "XmlExternalEntity", `
package test;

import javax.xml.parsers.DocumentBuilderFactory;

class XmlLoader {
    void load(java.io.InputStream input) throws Exception {
        DocumentBuilderFactory factory = DocumentBuilderFactory.newInstance();
        factory.newDocumentBuilder().parse(input);
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java finding, got %d", len(findings))
	}
}

func TestXmlExternalEntity_JavaNegative(t *testing.T) {
	findings := runRuleByNameOnJava(t, "XmlExternalEntity", `
package test;

import javax.xml.parsers.DocumentBuilderFactory;

class XmlLoader {
    void load(java.io.InputStream input) throws Exception {
        DocumentBuilderFactory factory = DocumentBuilderFactory.newInstance();
        factory.setFeature("http://apache.org/xml/features/disallow-doctype-decl", true);
        factory.newDocumentBuilder().parse(input);
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 Java findings, got %d", len(findings))
	}
}

func TestXmlExternalEntity_HardeningFormattingVariants(t *testing.T) {
	t.Run("multi-line setFeature kotlin", func(t *testing.T) {
		findings := runRuleByName(t, "XmlExternalEntity", `
package test

import javax.xml.parsers.DocumentBuilderFactory

class XmlLoader {
    fun load(input: java.io.InputStream) {
        val factory = DocumentBuilderFactory.newInstance()
        factory.setFeature(
            "http://apache.org/xml/features/disallow-doctype-decl",
            true,
        )
        factory.newDocumentBuilder().parse(input)
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for multi-line setFeature, got %d: %v", len(findings), findings)
		}
	})

	t.Run("extra whitespace around args kotlin", func(t *testing.T) {
		findings := runRuleByName(t, "XmlExternalEntity", `
package test

import javax.xml.parsers.DocumentBuilderFactory

class XmlLoader {
    fun load(input: java.io.InputStream) {
        val factory = DocumentBuilderFactory.newInstance()
        factory.setFeature( "http://apache.org/xml/features/disallow-doctype-decl" , true )
        factory.newDocumentBuilder().parse(input)
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for whitespaced setFeature, got %d: %v", len(findings), findings)
		}
	})

	t.Run("comment inside setFeature kotlin", func(t *testing.T) {
		findings := runRuleByName(t, "XmlExternalEntity", `
package test

import javax.xml.parsers.DocumentBuilderFactory

class XmlLoader {
    fun load(input: java.io.InputStream) {
        val factory = DocumentBuilderFactory.newInstance()
        factory.setFeature(
            // disable DOCTYPE
            "http://apache.org/xml/features/disallow-doctype-decl",
            true,
        )
        factory.newDocumentBuilder().parse(input)
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for commented setFeature, got %d: %v", len(findings), findings)
		}
	})

	t.Run("wrong boolean value still fires", func(t *testing.T) {
		findings := runRuleByName(t, "XmlExternalEntity", `
package test

import javax.xml.parsers.DocumentBuilderFactory

class XmlLoader {
    fun load(input: java.io.InputStream) {
        val factory = DocumentBuilderFactory.newInstance()
        factory.setFeature("http://apache.org/xml/features/disallow-doctype-decl", false)
        factory.newDocumentBuilder().parse(input)
    }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding when hardening boolean is wrong, got %d: %v", len(findings), findings)
		}
	})

	t.Run("multi-line setProperty kotlin", func(t *testing.T) {
		findings := runRuleByName(t, "XmlExternalEntity", `
package test

import javax.xml.stream.XMLInputFactory

class XmlLoader {
    fun load(input: java.io.InputStream) {
        val factory = XMLInputFactory.newInstance()
        factory.setProperty(
            XMLInputFactory.SUPPORT_DTD,
            false,
        )
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for multi-line setProperty, got %d: %v", len(findings), findings)
		}
	})

	t.Run("multi-line property assignment kotlin", func(t *testing.T) {
		findings := runRuleByName(t, "XmlExternalEntity", `
package test

import javax.xml.parsers.DocumentBuilderFactory

class XmlLoader {
    fun load(input: java.io.InputStream) {
        val factory = DocumentBuilderFactory.newInstance()
        factory.isXIncludeAware =
            false
        factory.isExpandEntityReferences =
            false
        factory.newDocumentBuilder().parse(input)
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for split property assignments, got %d: %v", len(findings), findings)
		}
	})

	t.Run("missing one property assignment still fires", func(t *testing.T) {
		findings := runRuleByName(t, "XmlExternalEntity", `
package test

import javax.xml.parsers.DocumentBuilderFactory

class XmlLoader {
    fun load(input: java.io.InputStream) {
        val factory = DocumentBuilderFactory.newInstance()
        factory.isXIncludeAware = false
        factory.newDocumentBuilder().parse(input)
    }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding when only isXIncludeAware is set, got %d: %v", len(findings), findings)
		}
	})

	t.Run("multi-line setFeature java", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "XmlExternalEntity", `
package test;

import javax.xml.parsers.DocumentBuilderFactory;

class XmlLoader {
    void load(java.io.InputStream input) throws Exception {
        DocumentBuilderFactory factory = DocumentBuilderFactory.newInstance();
        factory.setFeature(
            "http://apache.org/xml/features/disallow-doctype-decl",
            true
        );
        factory.newDocumentBuilder().parse(input);
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings for multi-line setFeature, got %d: %v", len(findings), findings)
		}
	})
}

func TestJavaObjectInputStream_KotlinPositive(t *testing.T) {
	findings := runRuleByName(t, "JavaObjectInputStream", `
package test

import java.io.FileInputStream
import java.io.ObjectInputStream

class Decoder {
    fun decode(path: String): Any {
        return ObjectInputStream(FileInputStream(path)).use { it.readObject() }
    }

    fun decodeQualified(input: java.io.InputStream): Any {
        return java.io.ObjectInputStream(input).readObject()
    }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %v", len(findings), findings)
	}
}

func TestJavaObjectInputStream_KotlinNegatives(t *testing.T) {
	t.Run("local lookalike and serialization library are clean", func(t *testing.T) {
		findings := runRuleByName(t, "JavaObjectInputStream", `
package test

class ObjectInputStream(input: Any)

class Decoder {
    fun decode(input: Any): ObjectInputStream {
        return ObjectInputStream(input)
    }

    fun json(text: String) = kotlinx.serialization.json.Json.decodeFromString<String>(text)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("test source is clean", func(t *testing.T) {
		findings := runRuleByNameOnPath(t, "JavaObjectInputStream", "src/test/kotlin/DecoderTest.kt", `
package test

import java.io.FileInputStream
import java.io.ObjectInputStream

class DecoderTest {
    fun fixture(path: String): Any = ObjectInputStream(FileInputStream(path)).readObject()
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("filtering subclass is clean", func(t *testing.T) {
		findings := runRuleByName(t, "JavaObjectInputStream", `
package test

import java.io.InputStream
import java.io.ObjectInputStream
import java.io.ObjectStreamClass

class FilteringInputStream(input: InputStream) : ObjectInputStream(input) {
    override fun resolveClass(desc: ObjectStreamClass): Class<*> {
        return super.resolveClass(desc)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestJavaObjectInputStream_JavaPositive(t *testing.T) {
	findings := runRuleByNameOnJava(t, "JavaObjectInputStream", `
package test;

import java.io.FileInputStream;
import java.io.ObjectInputStream;

class Decoder {
    Object decode(String path) throws Exception {
        return new ObjectInputStream(new FileInputStream(path)).readObject();
    }

    Object decodeQualified(java.io.InputStream input) throws Exception {
        return new java.io.ObjectInputStream(input).readObject();
    }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 Java findings, got %d: %v", len(findings), findings)
	}
}

func TestJavaObjectInputStream_JavaNegatives(t *testing.T) {
	findings := runRuleByNameOnJava(t, "JavaObjectInputStream", `
package test;

class ObjectInputStream {
    ObjectInputStream(Object input) {}
}

class Decoder {
    Object decode(Object input) {
        return new ObjectInputStream(input);
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
	}
}

func TestJavaObjectInputStream_KotlinStringLiteralMention(t *testing.T) {
	findings := runRuleByName(t, "JavaObjectInputStream", `
package test

fun warn() {
    println("warning: never use java.io.ObjectInputStream(input) directly")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestJavaObjectInputStream_JavaStringLiteralMention(t *testing.T) {
	findings := runRuleByNameOnJava(t, "JavaObjectInputStream", `
package test;

class Warn {
    void warn() {
        System.out.println("never use java.io.ObjectInputStream(input) directly");
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
	}
}

func TestJacksonDefaultTyping_KotlinPositive(t *testing.T) {
	findings := runRuleByName(t, "JacksonDefaultTyping", `
package test

import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.databind.jsontype.impl.LaissezFaireSubTypeValidator

class MapperFactory {
    fun unsafe(): ObjectMapper {
        val mapper = ObjectMapper()
        mapper.enableDefaultTyping()
        return ObjectMapper().activateDefaultTyping(LaissezFaireSubTypeValidator.instance)
    }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %v", len(findings), findings)
	}
}

func TestJacksonDefaultTyping_KotlinNegatives(t *testing.T) {
	findings := runRuleByName(t, "JacksonDefaultTyping", `
package test

import com.fasterxml.jackson.annotation.JsonTypeInfo
import com.fasterxml.jackson.databind.ObjectMapper

class LocalMapper {
    fun activateDefaultTyping() = Unit
}

@JsonTypeInfo(use = JsonTypeInfo.Id.NAME)
sealed class Event

class MapperFactory {
    fun safe(local: LocalMapper): ObjectMapper {
        local.activateDefaultTyping()
        return ObjectMapper().findAndRegisterModules()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestJacksonDefaultTyping_JavaPositive(t *testing.T) {
	findings := runRuleByNameOnJava(t, "JacksonDefaultTyping", `
package test;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.jsontype.impl.LaissezFaireSubTypeValidator;

class MapperFactory {
    ObjectMapper unsafe() {
        ObjectMapper mapper = new ObjectMapper();
        mapper.enableDefaultTyping();
        return new ObjectMapper().activateDefaultTyping(LaissezFaireSubTypeValidator.instance);
    }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 Java findings, got %d: %v", len(findings), findings)
	}
}

func TestJacksonDefaultTyping_JavaNegatives(t *testing.T) {
	findings := runRuleByNameOnJava(t, "JacksonDefaultTyping", `
package test;

import com.fasterxml.jackson.annotation.JsonTypeInfo;
import com.fasterxml.jackson.databind.ObjectMapper;

class LocalMapper {
    void activateDefaultTyping() {}
}

@JsonTypeInfo(use = JsonTypeInfo.Id.NAME)
class Event {}

class MapperFactory {
    ObjectMapper safe(LocalMapper local) {
        local.activateDefaultTyping();
        return new ObjectMapper().findAndRegisterModules();
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
	}
}

func TestGsonPolymorphicFromJson_KotlinPositive(t *testing.T) {
	findings := runRuleByName(t, "GsonPolymorphicFromJson", `
package test
import com.google.gson.Gson
import com.google.gson.GsonBuilder

fun parse(raw: String) {
    Gson().fromJson(raw, Any::class.java)
    val gson = Gson()
    gson.fromJson(raw, Object::class.java)
    GsonBuilder().create().fromJson(raw, kotlin.Any::class)
}
`)
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %d: %v", len(findings), findings)
	}
}

func TestGsonPolymorphicFromJson_KotlinNegatives(t *testing.T) {
	findings := runRuleByName(t, "GsonPolymorphicFromJson", `
package test
import com.google.gson.Gson

data class User(val id: String)
class LocalGson {
    fun fromJson(raw: String, type: Any) = Any()
}

fun parse(raw: String, local: LocalGson, type: Class<*>) {
    Gson().fromJson(raw, User::class.java)
    Gson().fromJson(raw, type)
    local.fromJson(raw, Any::class.java)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}

	lookalike := runRuleByName(t, "GsonPolymorphicFromJson", `
package test
class Gson {
    fun fromJson(raw: String, type: Any) = Any()
}
fun parse(raw: String) {
    Gson().fromJson(raw, Any::class.java)
}
`)
	if len(lookalike) != 0 {
		t.Fatalf("expected 0 local-lookalike findings, got %d: %v", len(lookalike), lookalike)
	}
}

func TestGsonPolymorphicFromJson_JavaPositive(t *testing.T) {
	findings := runRuleByNameOnJava(t, "GsonPolymorphicFromJson", `
package test;
import com.google.gson.Gson;
import com.google.gson.GsonBuilder;

class Parser {
    void parse(String raw) {
        new Gson().fromJson(raw, Object.class);
        Gson gson = new Gson();
        gson.fromJson(raw, java.lang.Object.class);
        new GsonBuilder().create().fromJson(raw, Object.class);
    }
}
`)
	if len(findings) != 3 {
		t.Fatalf("expected 3 Java findings, got %d: %v", len(findings), findings)
	}
}

func TestGsonPolymorphicFromJson_JavaNegatives(t *testing.T) {
	findings := runRuleByNameOnJava(t, "GsonPolymorphicFromJson", `
package test;
import com.google.gson.Gson;

class User {}
class LocalGson {
    Object fromJson(String raw, Class<?> type) { return new Object(); }
}
class Parser {
    void parse(String raw, LocalGson local, Class<?> type) {
        new Gson().fromJson(raw, User.class);
        new Gson().fromJson(raw, type);
        local.fromJson(raw, Object.class);
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
	}

	lookalike := runRuleByNameOnJava(t, "GsonPolymorphicFromJson", `
package test;
class Gson {
    Object fromJson(String raw, Class<?> type) { return new Object(); }
}
class Parser {
    void parse(String raw) {
        new Gson().fromJson(raw, Object.class);
    }
}
`)
	if len(lookalike) != 0 {
		t.Fatalf("expected 0 Java local-lookalike findings, got %d: %v", len(lookalike), lookalike)
	}
}

func TestTempFileWorldReadable_Positive_Kotlin(t *testing.T) {
	findings := runRuleByName(t, "TempFileWorldReadable", `
package test

import java.io.File

fun makeReadable() {
    val t = File.createTempFile("secret", ".txt")
    t.setReadable(true, false)
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "world-accessible") {
		t.Fatalf("expected world-accessible guidance, got %q", findings[0].Message)
	}
}

func TestTempFileWorldReadable_Positive_FilesToFile(t *testing.T) {
	findings := runRuleByName(t, "TempFileWorldReadable", `
package test

import java.nio.file.Files

fun makeReadable() {
    val t = Files.createTempFile("secret", ".txt").toFile()
    t.setWritable(true, false)
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestTempFileWorldReadable_Negative_OwnerOnlyTrue(t *testing.T) {
	findings := runRuleByName(t, "TempFileWorldReadable", `
package test

import java.io.File

fun makeReadable() {
    val t = File.createTempFile("secret", ".txt")
    t.setReadable(true, true)
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestTempFileWorldReadable_Negative_SingleArg(t *testing.T) {
	findings := runRuleByName(t, "TempFileWorldReadable", `
package test

import java.io.File

fun makeReadable() {
    val t = File.createTempFile("secret", ".txt")
    t.setReadable(true)
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestTempFileWorldReadable_Negative_NotFromCreateTempFile(t *testing.T) {
	findings := runRuleByName(t, "TempFileWorldReadable", `
package test

import java.io.File

fun makeReadable() {
    val t = File("/tmp/known.txt")
    t.setReadable(true, false)
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestTempFileWorldReadable_Negative_OtherFunctionBinding(t *testing.T) {
	findings := runRuleByName(t, "TempFileWorldReadable", `
package test

import java.io.File

fun bind() {
    val t = File.createTempFile("secret", ".txt")
    println(t)
}

fun makeReadable() {
    val t = File("/tmp/known.txt")
    t.setReadable(true, false)
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings (binding lookup must not cross function boundary), got %d", len(findings))
	}
}

func TestTempFileWorldReadable_Java_Positive(t *testing.T) {
	findings := runRuleByNameOnJava(t, "TempFileWorldReadable", `
package test;
import java.io.File;
import java.io.IOException;
class C {
    void m() throws IOException {
        File t = File.createTempFile("a", ".txt");
        t.setReadable(true, false);
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java finding, got %d", len(findings))
	}
}

func TestTempFileWorldReadable_Java_Negative(t *testing.T) {
	findings := runRuleByNameOnJava(t, "TempFileWorldReadable", `
package test;
import java.io.File;
import java.io.IOException;
class C {
    void ownerOnly() throws IOException {
        File t = File.createTempFile("a", ".txt");
        t.setReadable(true, true);
    }
    void notTemp() {
        File t = new File("/tmp/known.txt");
        t.setReadable(true, false);
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 Java findings, got %d", len(findings))
	}
}

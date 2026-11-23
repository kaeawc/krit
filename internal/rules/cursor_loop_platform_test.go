package rules_test

import "testing"

func TestCursorLoopPlatformColumns(t *testing.T) {
	cases := []struct {
		name   string
		header string
		column string
		want   int
	}{
		{"nested SDK class", "import android.provider.ContactsContract", "ContactsContract.CommonDataKinds.Contactables.CONTACT_ID", 1},
		{"imported SDK class", "import android.provider.ContactsContract.Contacts", "Contacts.LOOKUP_KEY", 1},
		{"fully qualified SDK", "", "android.provider.ContactsContract.Contacts.LOOKUP_KEY", 1},
		{"Java platform", "import java.util.jar.JarFile", "JarFile.MANIFEST_NAME", 1},
		{"fully qualified Java platform", "", "java.util.jar.JarFile.MANIFEST_NAME", 1},
		{"javax platform", "import javax.xml.XMLConstants", "XMLConstants.XML_NS_URI", 1},
		{"fully qualified javax platform", "", "javax.xml.XMLConstants.XML_NS_URI", 1},
		{"alias", "import android.provider.ContactsContract as Provider", "Provider.Contacts.LOOKUP_KEY", 1},
		{"conflicting alias", "import android.provider.ContactsContract as Provider\nimport example.Columns as Provider", "Provider.Contacts.LOOKUP_KEY", 0},
		{"conflicting last segment with alias", "import android.provider.ContactsContract.Contacts\nimport example.Contacts as Other", "Contacts.LOOKUP_KEY", 0},
		{"platform root aliased away", "import android.provider.ContactsContract.Contacts as Other", "Contacts.LOOKUP_KEY", 0},
		{"conflicting platform root", "import android.provider.ContactsContract.Contacts\nimport java.example.Contacts", "Contacts.LOOKUP_KEY", 0},
		{"unrelated plain import", "import android.provider.ContactsContract.Contacts\nimport example.Other", "Contacts.LOOKUP_KEY", 1},
		{"platform wildcard with explicit import", "import android.provider.ContactsContract.Contacts\nimport java.util.*", "Contacts.LOOKUP_KEY", 1},
		{"fully qualified root import conflict", "import example.android", "android.provider.ContactsContract.Contacts.LOOKUP_KEY", 0},
		{"unimported class", "", "Contacts.LOOKUP_KEY", 0},
		{"wildcard import", "import android.provider.ContactsContract.*", "Contacts.LOOKUP_KEY", 0},
		{"nonplatform import", "import example.Contacts", "Contacts.LOOKUP_KEY", 0},
		{"platform lookalike", "import androidx.provider.Contacts", "Contacts.LOOKUP_KEY", 0},
		{"unqualified constant", "import android.provider.ContactsContract.Contacts.LOOKUP_KEY", "LOOKUP_KEY", 0},
		{"lowercase field", "import android.provider.ContactsContract.Contacts", "Contacts.lookupKey", 0},
		{"computed field", "import android.provider.ContactsContract.Contacts", "Contacts.LOOKUP_KEY + suffix", 0},
		{"safe navigation", "import android.provider.ContactsContract.Contacts", "Contacts?.LOOKUP_KEY", 0},
		{"call receiver", "import android.provider.ContactsContract.Contacts", "Contacts().LOOKUP_KEY", 0},
		{"indexed receiver", "import android.provider.ContactsContract.Contacts", "Contacts[0].LOOKUP_KEY", 0},
		{"lowercase fully qualified lookalike", "", "androidx.provider.Contacts.LOOKUP_KEY", 0},
		{"camel case constant", "import android.provider.ContactsContract.Contacts", "Contacts.LookupKey", 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			code := test.header + `
fun read(db: Db) {
    db.query()?.use { rowCursor ->
        while (rowCursor.moveToNext()) {
            val id =
                rowCursor.getString(
                    rowCursor.getColumnIndexOrThrow(` + test.column + `)
                )
        }
    }
}`
			if findings := runRuleByName(t, "CursorLoopWithColumnIndexInLoop", code); len(findings) != test.want {
				t.Fatalf("expected %d, got %v", test.want, findings)
			}
		})
	}
}

func TestCursorLoopPlatformColumnShadows(t *testing.T) {
	cases := []string{
		`import android.database.Cursor
fun read(holder: Holder, cursor: Cursor) {
    holder.rows(cursor) { c -> while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }
}`,
		`import com.example.Columns.Contacts
fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }`,
		`import com.example.Columns.Other as Contacts
fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }`,
		`import com.example.Columns.*
fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }`,
		`fun read(holder: Holder, cursor: Cursor) { holder.rows(cursor) { outer -> cursor.use { c -> while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) } } }`,
		`fun read(source: Source) { ({ val c: Cursor = source; while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }).use { action -> action() } }`,
		`class Reader { companion object Contacts { var LOOKUP_KEY = "lookup" }; fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) } }`,
		"class Reader {\n    object Contacts { var LOOKUP_KEY = \"lookup\" }\n    fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }\n}",
		"class Reader {\n    object `Contacts` { var LOOKUP_KEY = \"lookup\" }\n    fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }\n}",
		`class Reader { object Contacts { var LOOKUP_KEY = "lookup" }; fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) } }`,
		`class Reader(private val Contacts: VariableContactColumns) { fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) } }; class VariableContactColumns(var LOOKUP_KEY: String)`,
		`class Reader { val Contacts = source; fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) } }`,
		`class Reader { companion object { val Contacts = source }; fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) } }`,
		`class Reader { class Contacts { companion object { var LOOKUP_KEY = "lookup" } }; fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) } }`,
		`fun unrelated(Contacts: Any) {}; fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }`,
		`fun unrelated() { val Contacts = source }; fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }`,
		`fun read(c: Cursor, holder: Holder) { with(holder) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) } }`,
		`fun read(c: Cursor, holder: Holder) { with(holder) { val rows = c; while (rows.moveToNext()) rows.getColumnIndex(Contacts.LOOKUP_KEY) } }`,
		`fun read(c: Cursor, holder: Holder) { holder.apply { val rows = c; while (rows.moveToNext()) rows.getColumnIndex(Contacts.LOOKUP_KEY) } }`,
		`fun read(c: Cursor, holder: Holder) { holder.run { val rows = c; while (rows.moveToNext()) rows.getColumnIndex(Contacts.LOOKUP_KEY) } }`,
		`fun read(holder: Holder) { holder.apply { c: Cursor -> while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) } }`,
		`fun read(holder: Holder) { holder.run { c: Cursor -> while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) } }`,
		`fun Holder.read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }`,
		`fun Holder?.read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }`,
		`val action = fun Holder.(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }`,
		`fun read(holder: Holder) { val action: Holder.() -> Unit = { fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) } }; holder.action() }`,
		`fun read(c: Cursor, Contacts: Any) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }`,
		`fun read(c: Cursor) { val Contacts = source; while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }`,
		`fun read(c: Cursor) { while (c.moveToNext()) { val Contacts = source; c.getColumnIndex(Contacts.LOOKUP_KEY) } }`,
		`fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY); val Contacts = source }`,
		`fun read(c: Cursor, LOOKUP_KEY: String) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }`,
		`fun read(c: Cursor) { val LOOKUP_KEY = source; while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }`,
		`fun read(c: Cursor, android: Any) { while (c.moveToNext()) c.getColumnIndex(android.provider.Contacts.LOOKUP_KEY) }`,
		`fun read(c: Cursor, provider: Any) { while (c.moveToNext()) c.getColumnIndex(android.provider.Contacts.LOOKUP_KEY) }`,
		`fun read(c: Cursor) { fun Contacts() = source; while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }`,
		`fun read(c: Cursor) { class Contacts; while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }`,
		`fun read(c: Cursor) { sources.forEach { Contacts -> while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) } }`,
		`fun read(c: Cursor) { try { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) } catch (Contacts: Exception) {} }`,
		`fun read(c: Cursor) { val (Contacts, other) = source; while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }`,
		`fun read(c: Cursor) { val (other, LOOKUP_KEY) = source; while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }`,
		`fun read(c: Cursor) { for (Contacts in sources) {} ; while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }`,
		`fun <Contacts> read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }`,
	}
	for _, code := range cases {
		t.Run(code, func(t *testing.T) {
			if findings := runRuleByName(t, "CursorLoopWithColumnIndexInLoop", "import android.provider.ContactsContract.Contacts\n"+code); len(findings) != 0 {
				t.Fatalf("shadowed platform column must stay silent: %v", findings)
			}
		})
	}
}

func TestCursorLoopPlatformLambdaAllowlist(t *testing.T) {
	for _, name := range []string{"use", "let", "also", "forEach", "forEachIndexed", "onEach", "map", "mapNotNull", "filter", "takeIf", "takeUnless", "repeat", "rows", "run", "apply", "with"} {
		t.Run(name, func(t *testing.T) {
			code := `import android.provider.ContactsContract.Contacts
fun read(source: Source) { source.` + name + ` { c: Cursor -> while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) } }`
			want := 1
			switch name {
			case "rows", "run", "apply", "with":
				want = 0
			}
			if findings := runRuleByName(t, "CursorLoopWithColumnIndexInLoop", code); len(findings) != want {
				t.Fatalf("expected %d, got %v", want, findings)
			}
		})
	}
	for _, body := range []string{
		`source.let { outer -> outer.use { c: Cursor -> while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) } }`,
		`source.use { val c: Cursor = source; while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }`,
		`repeat(2, { index -> val c: Cursor = source; while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) })`,
	} {
		t.Run(body, func(t *testing.T) {
			code := "import android.provider.ContactsContract.Contacts\nfun read(source: Source) { " + body + " }"
			if findings := runRuleByName(t, "CursorLoopWithColumnIndexInLoop", code); len(findings) != 1 {
				t.Fatalf("allowlisted lambda must report: %v", findings)
			}
		})
	}
}

func TestCursorLoopPlatformInheritedMemberKnownGap(t *testing.T) {
	code := `import android.provider.ContactsContract.Contacts
import example.BaseReader
class Reader : BaseReader() {
    fun read(c: Cursor) { while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }
}`
	if findings := runRuleByName(t, "CursorLoopWithColumnIndexInLoop", code); len(findings) != 1 {
		t.Fatalf("known gap: BaseReader in another project file can declare a mutable Contacts member: %v", findings)
	}
}

func TestCursorLoopPlatformNamedReceiverLambdaMemberKnownGap(t *testing.T) {
	findings := runRuleByNameMultiFile(t, "CursorLoopWithColumnIndexInLoop", map[string]string{
		"Holder.kt": `class Columns { var LOOKUP_KEY = "lookup" }
class Holder { val Contacts = Columns(); fun use(block: Holder.(Cursor) -> Unit) {} }`,
		"Reader.kt": `import android.database.Cursor
import android.provider.ContactsContract.Contacts
fun read(holder: Holder) {
    holder.use { c -> while (c.moveToNext()) c.getColumnIndex(Contacts.LOOKUP_KEY) }
	}`,
	})
	if len(findings) != 1 {
		t.Fatalf("known gap: a member function named use with a receiver lambda can bind Contacts: %v", findings)
	}
}

func TestCursorLoopJavaPlatformColumnShadows(t *testing.T) {
	cases := []string{
		`import static com.example.Columns.Contacts;
class Reader { void read(Cursor c) { while (c.moveToNext()) { c.getColumnIndex(Contacts.LOOKUP_KEY); } } }`,
		`import static com.example.Columns.*;
class Reader { void read(Cursor c) { while (c.moveToNext()) { c.getColumnIndex(Contacts.LOOKUP_KEY); } } }`,
		`import com.example.Columns.Contacts;
class Reader { void read(Cursor c) { while (c.moveToNext()) { c.getColumnIndex(Contacts.LOOKUP_KEY); } } }`,
		`class Reader { interface Contacts { String LOOKUP_KEY = pick(); } void read(Cursor c) { while (c.moveToNext()) { c.getColumnIndex(Contacts.LOOKUP_KEY); } } }`,
		`class Reader { @interface Contacts {} void read(Cursor c) { while (c.moveToNext()) { c.getColumnIndex(Contacts.LOOKUP_KEY); } } }`,
		`class Reader { private final Columns Contacts = new Columns(); void read(Cursor c) { while (c.moveToNext()) { c.getColumnIndex(Contacts.LOOKUP_KEY); } } } class Columns { String LOOKUP_KEY = "lookup"; }`,
		`class Reader { static class Contacts { static String LOOKUP_KEY = pick(); } void read(Cursor c) { while (c.moveToNext()) { c.getColumnIndex(Contacts.LOOKUP_KEY); } } }`,
		`class Other { Object Contacts; } class Reader { void read(Cursor c) { while (c.moveToNext()) { c.getColumnIndex(Contacts.LOOKUP_KEY); } } }`,
		`class Reader { void unrelated(Object Contacts) {} void read(Cursor c) { while (c.moveToNext()) { c.getColumnIndex(Contacts.LOOKUP_KEY); } } }`,
		`class Reader { void unrelated() { Object Contacts = source; } void read(Cursor c) { while (c.moveToNext()) { c.getColumnIndex(Contacts.LOOKUP_KEY); } } }`,
	}
	for _, code := range cases {
		t.Run(code, func(t *testing.T) {
			if findings := runRuleByNameOnJava(t, "CursorLoopWithColumnIndexInLoop", "import android.provider.ContactsContract.Contacts;\n"+code); len(findings) != 0 {
				t.Fatalf("shadowed platform column must stay silent: %v", findings)
			}
		})
	}
}

func TestCursorLoopJavaPlatformColumns(t *testing.T) {
	cases := []struct {
		header string
		column string
		params string
		want   int
	}{
		{"import android.provider.ContactsContract;", "ContactsContract.CommonDataKinds.Contactables.CONTACT_ID", "", 1},
		{"import android.provider.ContactsContract.Contacts;", "Contacts.LOOKUP_KEY", "", 1},
		{"import android.provider.ContactsContract.Contacts;\nimport com.example.model.*;", "Contacts.LOOKUP_KEY", "", 1},
		{"import static com.example.Columns.Contacts;\nimport android.provider.ContactsContract.Contacts;", "Contacts.LOOKUP_KEY", "", 0},
		{"import static com.example.Columns.*;\nimport android.provider.ContactsContract.Contacts;", "Contacts.LOOKUP_KEY", "", 0},
		{"import static com.example.Columns.OTHER;\nimport android.provider.ContactsContract.Contacts;", "Contacts.LOOKUP_KEY", "", 1},
		{"", "android.provider.ContactsContract.Contacts.LOOKUP_KEY", "", 1},
		{"import java.util.*;", "android.provider.ContactsContract.Contacts.LOOKUP_KEY", "", 1},
		{"import com.example.*;", "android.provider.ContactsContract.Contacts.LOOKUP_KEY", "", 0},
		{"import java.util.jar.JarFile;", "JarFile.MANIFEST_NAME", "", 1},
		{"import example.Contacts;", "Contacts.LOOKUP_KEY", "", 0},
		{"import android.provider.ContactsContract.*;", "Contacts.LOOKUP_KEY", "", 0},
		{"import android.provider.ContactsContract.Contacts;", "Contacts.LOOKUP_KEY", ", Object Contacts", 0},
		{"import android.provider.ContactsContract.Contacts;", "Contacts.LOOKUP_KEY", ", String LOOKUP_KEY", 0},
	}
	for _, test := range cases {
		t.Run(test.header+test.column+test.params, func(t *testing.T) {
			code := test.header + ` class Reader { void read(Cursor c` + test.params + `) {
while (c.moveToNext()) { String value = c.getString(c.getColumnIndexOrThrow(` + test.column + `)); }
} }`
			if findings := runRuleByNameOnJava(t, "CursorLoopWithColumnIndexInLoop", code); len(findings) != test.want {
				t.Fatalf("expected %d, got %v", test.want, findings)
			}
		})
	}
}

func TestCursorLoopContactsQuerySources(t *testing.T) {
	bodies := []string{
		`queryRows(limit, offset)?.use { rowCursor ->
while (rowCursor.moveToNext()) {
val id = rowCursor.getString(rowCursor.getColumnIndexOrThrow(ContactsContract.CommonDataKinds.Contactables.CONTACT_ID))
}
}`,
		`resolver.query(Contacts.CONTENT_URI, null, null, null, null)?.use { cursor ->
while (cursor.moveToNext()) {
val key = cursor.getString(cursor.getColumnIndexOrThrow(Contacts.LOOKUP_KEY))
}
}`,
	}
	for _, function := range []string{"loadContacts", "testContacts"} {
		for _, body := range bodies {
			t.Run(function+body, func(t *testing.T) {
				code := `import android.provider.ContactsContract
import android.provider.ContactsContract.Contacts
fun ` + function + `(resolver: ContentResolver, queryRows: (Int, Int) -> Cursor?, limit: Int, offset: Int) {
` + body + `
}`
				findings := runRuleByName(t, "CursorLoopWithColumnIndexInLoop", code)
				if len(findings) != 1 || findings[0].Line != 6 {
					t.Fatalf("expected one SDK lookup finding on line 6, got %v", findings)
				}
			})
		}
	}
}

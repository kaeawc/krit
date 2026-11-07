package rules_test

import "testing"

func TestAuditResourceCost(t *testing.T) {
	runAuditCases(t, []auditCase{
		{"cursor dynamic column", "CursorLoopWithColumnIndexInLoop", `
package test
import android.database.Cursor
fun f(cursor: Cursor, columns: List<String>) {
    while (cursor.moveToNext()) {
        for (c in columns) {
            println(cursor.getString(cursor.getColumnIndexOrThrow(c)))
        }
    }
}
`, 0},
		{"cursor constant column", "CursorLoopWithColumnIndexInLoop", `
package test
import android.database.Cursor
fun f(cursor: Cursor) {
    while (cursor.moveToNext()) {
        println(cursor.getString(cursor.getColumnIndexOrThrow("name")))
    }
}
`, 1},
		{"buffered read big buffer", "BufferedReadWithoutBuffer", `
package test
import java.io.FileInputStream
fun f(path: String) {
    val fis = FileInputStream(path)
    val buf = ByteArray(65536)
    fis.read(buf)
}
`, 0},
		{"single byte read", "BufferedReadWithoutBuffer", `
package test
import java.io.FileInputStream
fun f(path: String) {
    val fis = FileInputStream(path)
    var b = fis.read()
    while (b != -1) { b = fis.read() }
}
`, 2},
		{"okhttp derived client", "OkHttpClientCreatedPerCall", `
package test
import okhttp3.OkHttpClient
class Api(private val base: OkHttpClient) {
    fun fetch() { val c = base.newBuilder().build(); println(c) }
}
`, 0},
		{"okhttp new in function", "OkHttpClientCreatedPerCall", `
package test
import okhttp3.OkHttpClient
class Api {
    fun fetch() { val c = OkHttpClient(); println(c) }
}
`, 1},
		{"okhttp lazy property", "OkHttpClientCreatedPerCall", `
package test
import okhttp3.OkHttpClient
class Api {
    val client by lazy { OkHttpClient() }
}
`, 0},
		{"okhttp execute in suspend", "OkHttpCallExecuteSync", `
package test
import okhttp3.Call
suspend fun f(call: Call) { call.execute() }
`, 1},
		{"okhttp execute with context", "OkHttpCallExecuteSync", `
package test
import okhttp3.Call
import kotlinx.coroutines.*
suspend fun f(call: Call) = withContext(Dispatchers.IO) { call.execute() }
`, 0},
		{"execute unrelated command", "OkHttpCallExecuteSync", `
package test
class Command { fun execute() {} }
suspend fun f(command: Command) { command.execute() }
`, 0},
		{"retrofit create in function", "RetrofitCreateInHotPath", `
package test
import retrofit2.Retrofit
interface Api
fun f(): Api = Retrofit.Builder().baseUrl("http://x").build().create(Api::class.java)
`, 1},
		{"retrofit create in provides", "RetrofitCreateInHotPath", `
package test
import retrofit2.Retrofit
annotation class Provides
interface Api
@Provides
fun f(r: Retrofit): Api = r.create(Api::class.java)
`, 0},
		{"workmanager periodic 5min", "PeriodicWorkRequestLessThan15Min", `
package test
import androidx.work.PeriodicWorkRequestBuilder
import java.util.concurrent.TimeUnit
class W : androidx.work.ListenableWorker
fun f() { PeriodicWorkRequestBuilder<W>(5, TimeUnit.MINUTES) }
`, 1},
		{"workmanager periodic hours", "PeriodicWorkRequestLessThan15Min", `
package test
import androidx.work.PeriodicWorkRequestBuilder
import java.util.concurrent.TimeUnit
class W : androidx.work.ListenableWorker
fun f() { PeriodicWorkRequestBuilder<W>(1, TimeUnit.HOURS) }
`, 0},
		{"workmanager periodic seconds 900", "PeriodicWorkRequestLessThan15Min", `
package test
import androidx.work.PeriodicWorkRequestBuilder
import java.util.concurrent.TimeUnit
class W : androidx.work.ListenableWorker
fun f() { PeriodicWorkRequestBuilder<W>(900, TimeUnit.SECONDS) }
`, 0},
		{"workmanager periodic duration", "PeriodicWorkRequestLessThan15Min", `
package test
import androidx.work.PeriodicWorkRequestBuilder
import java.time.Duration
class W : androidx.work.ListenableWorker
fun f() { PeriodicWorkRequestBuilder<W>(Duration.ofMinutes(10)) }
`, 1},
		{"workmanager periodic days", "PeriodicWorkRequestLessThan15Min", `
package test
import androidx.work.PeriodicWorkRequestBuilder
import java.util.concurrent.TimeUnit
class W : androidx.work.ListenableWorker
fun f() { PeriodicWorkRequestBuilder<W>(1, TimeUnit.DAYS) }
`, 0},
	})
}

func TestAuditResourceCostCompose(t *testing.T) {
	runAuditCases(t, []auditCase{
		{"scroll column with lazy", "LazyColumnInsideColumn", `
package test
fun f() {
    Column(modifier = Modifier.verticalScroll(rememberScrollState())) {
        LazyColumn { }
    }
}
`, 1},
		{"scroll on sibling only", "LazyColumnInsideColumn", `
package test
fun f() {
    Column {
        Box(Modifier.verticalScroll(rememberScrollState())) { }
        LazyColumn { }
    }
}
`, 0},
		{"bounded lazy column", "LazyColumnInsideColumn", `
package test
fun f() {
    Column(modifier = Modifier.verticalScroll(rememberScrollState())) {
        LazyColumn(modifier = Modifier.height(200.dp)) { }
    }
}
`, 0},
		{"lazy row in horizontal scroll row", "LazyColumnInsideColumn", `
package test
fun f() {
    Row(modifier = Modifier.horizontalScroll(rememberScrollState())) {
        LazyRow { }
    }
}
`, 1},
		{"remember keyed", "ComposeRememberInList", `
package test
fun f(list: List<String>) {
    LazyColumn {
        items(list) { item ->
            val label = remember(item) { item.uppercase() }
        }
    }
}
`, 0},
		{"remember unkeyed uses item", "ComposeRememberInList", `
package test
fun f(list: List<String>) {
    LazyColumn {
        items(list) { item ->
            val label = remember { item.uppercase() }
        }
    }
}
`, 1},
		{"remember unkeyed not using item", "ComposeRememberInList", `
package test
fun f(list: List<String>) {
    LazyColumn {
        items(list) { item ->
            val state = remember { mutableStateOf(false) }
        }
    }
}
`, 0},
		{"painter in items constant", "ComposePainterResourceInLoop", `
package test
fun f(list: List<String>) {
    LazyColumn {
        items(list) { item ->
            Image(painter = painterResource(R.drawable.icon), contentDescription = null)
        }
    }
}
`, 1},
		{"painter in items depends on item", "ComposePainterResourceInLoop", `
package test
fun f(list: List<Int>) {
    LazyColumn {
        items(list) { item ->
            Image(painter = painterResource(item), contentDescription = null)
        }
    }
}
`, 0},
		{"recycler in lazy column", "RecyclerViewInLazyColumn", `
package test
fun f(list: List<String>) {
    LazyColumn {
        items(list) { AndroidView(factory = { ctx -> RecyclerView(ctx) }) }
    }
}
`, 1},
		{"android view non recycler", "RecyclerViewInLazyColumn", `
package test
fun f(list: List<String>) {
    LazyColumn {
        items(list) { AndroidView(factory = { ctx -> TextView(ctx) }) }
    }
}
`, 0},
		{"workmanager backoff missing", "WorkManagerNoBackoff", `
package test
fun f() {
    OneTimeWorkRequestBuilder<W>().build()
}
`, 1},
		{"workmanager backoff set", "WorkManagerNoBackoff", `
package test
fun f() {
    OneTimeWorkRequestBuilder<W>().setBackoffCriteria(BackoffPolicy.LINEAR, 10, TimeUnit.SECONDS).build()
}
`, 0},
		{"image glide skip memory", "ImageLoaderNoMemoryCache", `
package test
fun f() { Glide.with(c).load(u).skipMemoryCache(true).into(v) }
`, 1},
		{"image glide skip memory false", "ImageLoaderNoMemoryCache", `
package test
fun f() { Glide.with(c).load(u).skipMemoryCache(false).into(v) }
`, 0},
	})
}

func TestAuditDatabaseQueryOnMainThread(t *testing.T) {
	runAuditCases(t, []auditCase{
		{"click listener query", "DatabaseQueryOnMainThread", `
package test
import android.database.sqlite.SQLiteDatabase
class A(private val db: SQLiteDatabase) {
    fun wire(button: android.view.View) {
        button.setOnClickListener { db.rawQuery("select 1", null) }
    }
}
`, 1},
		{"io launch then main sibling", "DatabaseQueryOnMainThread", `
package test
import android.database.sqlite.SQLiteDatabase
import kotlinx.coroutines.*
class A(private val db: SQLiteDatabase, private val scope: CoroutineScope) {
    fun onClick() {
        scope.launch(Dispatchers.IO) {
            val c = db.rawQuery("select 1", null)
            withContext(Dispatchers.Main) { println(c) }
        }
    }
}
`, 0},
		{"main launch with io withContext", "DatabaseQueryOnMainThread", `
package test
import android.database.sqlite.SQLiteDatabase
import kotlinx.coroutines.*
class A(private val db: SQLiteDatabase, private val scope: CoroutineScope) {
    fun onClick() {
        scope.launch(Dispatchers.Main) {
            withContext(Dispatchers.IO) { db.rawQuery("select 1", null) }
        }
    }
}
`, 0},
	})
}

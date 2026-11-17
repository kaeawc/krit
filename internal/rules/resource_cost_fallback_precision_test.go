package rules_test

import (
	"strings"
	"testing"
)

// The Go rules below also run on files that do not compile, where a name can be imported from another library.

func TestOkHttpCallExecuteSync_NameGuessNeedsOkHttpInFile(t *testing.T) {
	cases := map[string]string{
		"another library's call type": `
package test
import com.example.net.HttpCall
suspend fun load(call: HttpCall) = call.execute()
`,
		"retrofit call": `
package test
import retrofit2.Call
suspend fun load(call: Call<String>) = call.execute()
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if findings := runRuleByName(t, "OkHttpCallExecuteSync", src); len(findings) != 0 {
				t.Fatalf("expected no finding without okhttp3 in the file, got %v", findings)
			}
		})
	}
	t.Run("okhttp client call is still reported", func(t *testing.T) {
		findings := runRuleByName(t, "OkHttpCallExecuteSync", `
package test
import okhttp3.OkHttpClient
import okhttp3.Request
class Loader(private val client: OkHttpClient) {
    suspend fun load(request: Request) = client.newCall(request).execute()
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %v", findings)
		}
	})
}

func TestOkHttpCallExecuteSync_InjectedIoDispatcherIsExempt(t *testing.T) {
	findings := runRuleByName(t, "OkHttpCallExecuteSync", `
package test
import kotlinx.coroutines.withContext
import okhttp3.Call
class Loader(private val appDispatchers: AppDispatchers) {
    suspend fun io(call: Call) = withContext(appDispatchers.io) { call.execute() }
    suspend fun default(call: Call) = withContext(dispatchers.default) { call.execute() }
    suspend fun main(call: Call) = withContext(appDispatchers.main) { call.execute() }
    suspend fun nested(call: Call) = withContext(appDispatchers.main) {
        withContext(appDispatchers.io) { prefetch() }
        call.execute()
    }
    suspend fun flowed(call: Call) = flow { emit(call.execute()) }.flowOn(appDispatchers.io)
    suspend fun sibling(call: Call) = run(appDispatchers.io, other { call.execute() })
}
`)
	if len(findings) != 2 || findings[0].Line != 8 || findings[1].Line != 11 {
		t.Fatalf("expected the main-dispatcher calls on lines 8 and 11, got %v", findings)
	}
}

func TestBufferedReadWithoutBuffer_ConstantLargeBuffers(t *testing.T) {
	findings := runRuleByName(t, "BufferedReadWithoutBuffer", `
package test
import java.io.File
import java.io.FileInputStream
private const val CHUNK = 16384
class Reader(private val file: File) {
    private val buffer = ByteArray(32768)
    fun defaultSize() = FileInputStream(file).read(ByteArray(DEFAULT_BUFFER_SIZE))
    fun product() = FileInputStream(file).read(ByteArray(8 * 1024))
    fun constant() = FileInputStream(file).read(ByteArray(CHUNK))
    fun member() = FileInputStream(file).read(buffer)
    fun hex() = FileInputStream(file).read(ByteArray(0x2000))
    fun small() = FileInputStream(file).read(ByteArray(4 * 1024))
}
`)
	if len(findings) != 1 || findings[0].Line != 13 {
		t.Fatalf("expected only the 4 KiB read on line 13, got %v", findings)
	}
}

func TestBufferedReadWithoutBuffer_OtherFileInputStreamImport(t *testing.T) {
	findings := runRuleByName(t, "BufferedReadWithoutBuffer", `
package test
import com.example.io.FileInputStream
fun read(path: String) = FileInputStream(path).read()
`)
	if len(findings) != 0 {
		t.Fatalf("expected no finding for another library's FileInputStream, got %v", findings)
	}
}

func TestComposePainterResourceInLoop_OtherPainterResourceImport(t *testing.T) {
	findings := runRuleByName(t, "ComposePainterResourceInLoop", `
package test
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import com.example.res.painterResource
fun list(rows: List<String>) {
    LazyColumn { items(rows) { row -> Icon(painterResource(R.drawable.a), row) } }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no finding for another library's painterResource, got %v", findings)
	}
}

func TestComposePainterResourceInLoop_InlineLoopInsideForLoop(t *testing.T) {
	findings := runRuleByName(t, "ComposePainterResourceInLoop", `
package test
import androidx.compose.foundation.layout.Column
import androidx.compose.ui.res.painterResource
fun grid(rows: List<String>) {
    Column {
        for (row in rows) {
            listOf(1, 2).forEach { Icon(painterResource(R.drawable.a), row) }
        }
    }
}
`)
	if len(findings) != 1 || findings[0].Line != 8 {
		t.Fatalf("expected the forEach inside a for loop to be reported on line 8, got %v", findings)
	}
}

func TestRecyclerAdapterWithoutDiffUtil_NestedAdapterCallIsReportedOnce(t *testing.T) {
	findings := runRuleByName(t, "RecyclerAdapterWithoutDiffUtil", `
package test
import androidx.recyclerview.widget.RecyclerView
class OuterAdapter : RecyclerView.Adapter<RecyclerView.ViewHolder>() {
    class InnerAdapter : RecyclerView.Adapter<RecyclerView.ViewHolder>() {
        fun refresh() { notifyDataSetChanged() }
    }
}
`)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "'InnerAdapter'") {
		t.Fatalf("expected one finding for InnerAdapter, got %v", findings)
	}
}

func TestRecyclerAdapterWithoutDiffUtil_AdapterThroughProjectBase(t *testing.T) {
	findings := runRuleByNameMultiFile(t, "RecyclerAdapterWithoutDiffUtil", map[string]string{
		"ResourcesBase.kt": `
package base
import androidx.recyclerview.widget.RecyclerView
abstract class ResourcesBase<VH : RecyclerView.ViewHolder> : RecyclerView.Adapter<VH>()
`,
		"Rows.kt": `
package rows
import androidx.recyclerview.widget.RecyclerView
import base.ResourcesBase
class Rows : ResourcesBase<RecyclerView.ViewHolder>() {
    class NestedRows : ResourcesBase<RecyclerView.ViewHolder>() {
        fun reload() { notifyDataSetChanged() }
    }
    fun refresh() { notifyDataSetChanged() }
}
`,
	})
	var names []string
	for _, f := range findings {
		names = append(names, f.Message[:strings.Index(f.Message, " uses")])
	}
	if strings.Join(names, ",") != "'Rows','NestedRows'" && strings.Join(names, ",") != "'NestedRows','Rows'" {
		t.Fatalf("expected one finding each for Rows and NestedRows, got %v", findings)
	}
}

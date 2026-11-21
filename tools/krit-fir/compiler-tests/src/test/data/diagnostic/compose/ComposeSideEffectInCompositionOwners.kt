// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 38, 42
package test

import androidx.compose.runtime.Composable
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import golden.preview.RenderVariants

class Holder(var value: Int = 0, var seen: Boolean = false)

// Both engines exempt enclosing plain locals, fresh remembered objects and delegated state in composable lambdas.
@Composable fun A() { var count = 0; Column { count = 1 } }
@Composable fun B() { val cache = mutableMapOf<String, Int>(); Column { cache["k"] = 1 } }
@Composable fun C(input: Int) { val h = remember { Holder() }; Column { h.value = input } }
@Composable fun D() { var x by remember { mutableStateOf(0) }; Box { x = 1 } }

// Both engines exempt a preview's lambda through the nearest named owner.
@Preview @Composable fun P(holder: Holder) { Column { holder.seen = true } }

// Both engines use the enclosing class TestHarness rather than the companion as test-double evidence.
class TestHarness {
    companion object {
        @Composable fun R(holder: Holder) { holder.seen = true }
    }
}

// Go ignores objects when looking for test-double names; FIR conservatively stays silent for FakeRepo.
object FakeRepo {
    @Composable fun R(holder: Holder) { holder.seen = true }
}

// Go cannot see the multipreview declared in a separate stub source file; FIR resolves it and stays silent.
@RenderVariants @Composable fun CrossFile(holder: Holder) { holder.seen = true }

fun RenderItem(content: @Composable (Int) -> Unit) {}

// Both engines stay silent for last* captures of lambda parameters; Go never inspects lambdas.
class Presenter {
    var lastInput = 0
    var lastItem = 0
    @Composable fun Capture() { RenderItem { input -> lastInput = input } }
    @Composable fun CaptureItems(list: List<Int>) { LazyColumn { items(list) { item -> lastItem = item } } }
    @Composable fun CaptureOuterLambda() { RenderItem { input -> Column { lastInput = input } } }
    @Composable fun CaptureOwner(input: Int) { RenderItem { Column { lastInput = input } } }
    @Composable fun CaptureThroughLocal(input: Int) {
        fun local() { RenderItem { Column { lastInput = input } } }
    }
}

// Both engines exempt a simple MutableState receiver, including an explicit this. qualifier.
class Screen(private val state: MutableState<Int>) {
    @Composable fun Render(input: Int) { this.state.value = input }
}

// Go's class_declaration covers interfaces, enums and annotations; both engines exempt these test-double owners.
interface TestScreens {
    companion object {
        @Composable fun R(holder: Holder) { holder.seen = true }
    }
}
enum class FakeMode {
    A;
    companion object {
        @Composable fun R(holder: Holder) { holder.seen = true }
    }
}
annotation class TestAnnotation {
    companion object {
        @Composable fun R(holder: Holder) { holder.seen = true }
    }
}

// Both engines stay silent for composable lambdas inside local classes; Go never inspects lambdas.
@Composable fun LocalClassContent(holder: Holder) {
    class Local {
        val content: @Composable () -> Unit = { holder.seen = true }
        init { Column { holder.seen = true } }
    }
}

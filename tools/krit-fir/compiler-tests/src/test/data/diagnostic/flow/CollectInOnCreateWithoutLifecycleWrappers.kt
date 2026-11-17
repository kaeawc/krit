// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 42, 43, 44, 47
// The calls between the lifecycle callback and the collect decide. A collect
// downstream of flowWithLifecycle, or passed into a call that may follow the
// lifecycle (repeatOnLifecycle, a project wrapper around it, a callback), is
// not reported. Inside the coroutine builders and scope functions, and the
// launchWhenX builders that only pause the block, it is. A builder the
// collect's launch is the receiver of does not wrap it. A chain split across
// lines is reported on its `.collect` line.
package test

import android.os.Bundle
import androidx.appcompat.app.AppCompatActivity
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleCoroutineScope
import androidx.lifecycle.LifecycleOwner
import androidx.lifecycle.flowWithLifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.launch

fun LifecycleCoroutineScope.launchWhileStarted(owner: LifecycleOwner, block: suspend CoroutineScope.() -> Unit): Job =
    launch { owner.repeatOnLifecycle(Lifecycle.State.STARTED) { block() } }

fun onClick(block: () -> Unit) = block()

class WrapperActivity : AppCompatActivity() {
    private val flow: Flow<Int> = TODO()
    private val scope: CoroutineScope = TODO()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        lifecycleScope.launch { flow.flowWithLifecycle(lifecycle, Lifecycle.State.STARTED).collect { } }
        lifecycleScope.launch { flow.flowWithLifecycle(lifecycle).map { it + 1 }.collect { } }
        lifecycleScope.launchWhileStarted(this) { flow.collect { } }
        onClick { scope.launch { flow.collect { } } }
        @Suppress("DEPRECATION")
        lifecycleScope.launchWhenStarted { flow.<!CollectInOnCreateWithoutLifecycle!>collect<!> { } }
        lifecycleScope.launch { flow.<!CollectInOnCreateWithoutLifecycle!>collect<!> { } }.invokeOnCompletion { }
        flow.let { upstream -> lifecycleScope.launch { launch { upstream.<!CollectInOnCreateWithoutLifecycle!>collect<!> { } } } }
        lifecycleScope.launch {
            flow
                .<!CollectInOnCreateWithoutLifecycle!>collect<!> { }
        }
    }
}

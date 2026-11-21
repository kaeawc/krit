// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 22, 27
package scopeParentContexts

import kotlinx.coroutines.*
import kotlin.coroutines.CoroutineContext
import androidx.lifecycle.ViewModel
import androidx.lifecycle.LifecycleOwner
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.test.TestScope
import kotlin.coroutines.ContinuationInterceptor

class LocalJobParent {
    private val job = SupervisorJob()
    // Go reports; a resolved Job-typed value conservatively supplies ownership even without cancellation.
    val scope = CoroutineScope(job + Dispatchers.IO)
}

class ParameterJobParent(job: Job) {
    // Go reports; FIR conservatively accepts the resolved Job-typed parameter as a parent.
    val scope = CoroutineScope(job + Dispatchers.IO)
}

class ContextParameterParent(context: CoroutineContext) {
    // Go reports; FIR cannot rule out a parent Job inside an opaque CoroutineContext parameter.
    val scope = CoroutineScope(context)
}

class DispatcherElementIsNotAParent(parent: CoroutineScope) {
    // Go sees coroutineContext and stays silent; this contains only a dispatcher, so CoroutineScope creates a fresh Job.
    val <!CoroutineScopeCreatedButNeverCancelled!>scope<!> = CoroutineScope(parent.coroutineContext[ContinuationInterceptor]!!)
}

class ViewModelParent : ViewModel() {
    val scope = CoroutineScope(viewModelScope.coroutineContext + Dispatchers.IO)
}

class LifecycleParent(owner: LifecycleOwner) {
    val scope = CoroutineScope(owner.lifecycleScope.coroutineContext + Dispatchers.IO)
}

class BackgroundParent(owner: TestScope) {
    val scope = CoroutineScope(owner.backgroundScope.coroutineContext + Dispatchers.IO)
}

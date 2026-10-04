// Compiler-test source stubs; never packaged in the production artifact.
package kotlinx.coroutines.tasks

import com.google.android.gms.tasks.Task

suspend fun <T> Task<T>.await(): T = TODO()

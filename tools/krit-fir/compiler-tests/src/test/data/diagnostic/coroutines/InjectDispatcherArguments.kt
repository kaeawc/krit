// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 21, 22, 24
// A named argument, a fully qualified reference, and a limitedParallelism view
// of the dispatcher pass it as hardcoded as a lone argument; detekt reports
// all three. A parameter's default value is the injection point itself, so a
// dispatcher there is not reported (detekt skips parameters too).
package test

import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

class Loader(val dispatcher: CoroutineDispatcher)

class Repository(
    private val loader: Loader = Loader(Dispatchers.IO),
) {
    constructor(name: String, other: Loader = Loader(Dispatchers.Default)) : this(other)

    suspend fun named() = withContext(context = <!InjectDispatcher!>Dispatchers.IO<!>) { }
    suspend fun qualified() = withContext(<!InjectDispatcher!>kotlinx.coroutines.Dispatchers.Default<!>) { }
    suspend fun limited() = withContext(<!InjectDispatcher!>Dispatchers.IO<!>.limitedParallelism(2)) { }
    fun withDefault(other: Loader = Loader(Dispatchers.IO)) = other
    fun eager() = Loader(<!InjectDispatcher!>Dispatchers.IO<!>)
}

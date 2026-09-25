// Smoke: Context.current() and a Scope closed by use {}.
package stubs

import io.opentelemetry.context.Context
import io.opentelemetry.context.Scope

fun propagate(block: () -> Unit) {
    val context: Context = Context.current()
    val scope: Scope = context.makeCurrent()
    scope.use { block() }
    context.wrap(Runnable(block)).run()
}

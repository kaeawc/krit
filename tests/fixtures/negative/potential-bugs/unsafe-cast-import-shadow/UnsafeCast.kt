package demo.feature

import android.app.Application
import android.content.Context

// The nested Slot.Context is not in scope here: `Context` is the imported android.content.Context,
// and an Application is a Context, so the casts below can succeed.
sealed interface Slot {
    data class Context(val label: String) : Slot
}

class DemoApp : Application()

class RegistrarProvider(private val context: Context) {
    fun app(): DemoApp = context as DemoApp

    fun appOrNull(): DemoApp? = context as? DemoApp
}

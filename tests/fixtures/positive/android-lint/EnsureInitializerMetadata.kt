package fixtures.androidlint

import android.content.Context
import androidx.startup.Initializer

// No manifest <meta-data> entry and no dependencies() list names this Initializer.
class OrphanInitializer : Initializer<Unit> {
    override fun create(context: Context) = Unit
    override fun dependencies(): List<Class<out Initializer<*>>> = emptyList()
}

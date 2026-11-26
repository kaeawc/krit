package fixtures.androidlint

import android.content.Context
import androidx.startup.Initializer

// Listed in the manifest as <meta-data android:name="fixtures.androidlint.RootInitializer" android:value="androidx.startup"/>.
class RootInitializer : Initializer<Unit> {
    override fun create(context: Context) = Unit
    override fun dependencies(): List<Class<out Initializer<*>>> = listOf(ChildInitializer::class.java)
}

class ChildInitializer : Initializer<Unit> {
    override fun create(context: Context) = Unit
    override fun dependencies(): List<Class<out Initializer<*>>> = emptyList()
}

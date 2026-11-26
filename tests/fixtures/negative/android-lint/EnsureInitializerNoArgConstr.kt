package fixtures.androidlint

import android.content.Context
import androidx.startup.Initializer

class AnalyticsInitializer : Initializer<Unit> {
    override fun create(context: Context) = Unit
    override fun dependencies(): List<Class<out Initializer<*>>> = emptyList()
}

class KeyedInitializer(private val key: String = "default") : Initializer<Unit> {
    override fun create(context: Context) = Unit
    override fun dependencies(): List<Class<out Initializer<*>>> = emptyList()
}

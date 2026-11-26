package fixtures.androidlint

import android.content.Context
import androidx.startup.Initializer

class AnalyticsInitializer(private val key: String) : Initializer<Unit> {
    override fun create(context: Context) = Unit
    override fun dependencies(): List<Class<out Initializer<*>>> = emptyList()
}

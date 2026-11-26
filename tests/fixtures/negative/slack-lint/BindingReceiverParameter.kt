package fixtures.negative.slacklint

import dagger.Module
import dagger.Provides

@Module
object NameModule {
    @Provides fun provideName(value: String): CharSequence = value
}

object Helpers {
    // Not a provider: extension functions are fine outside @Binds/@Provides.
    fun String.shout(): String = uppercase()
}

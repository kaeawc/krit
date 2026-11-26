package fixtures.positive.slacklint

import dagger.Module
import dagger.Provides

@Module
object NameModule {
    @Provides fun String.provideName(): CharSequence = this
}

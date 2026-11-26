package fixtures.negative.slacklint

import dagger.Module
import dagger.Provides

@Module
object NameModule {
    @Provides fun provideName(): String = "name"

    // The type of an expression body is not known from source, so the rule abstains.
    @Provides fun provideInferred() = "name"
}

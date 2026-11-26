package fixtures.positive.slacklint

import dagger.Module
import dagger.Provides

@Module
abstract class NameModule {
    @Provides abstract fun provideName(): String
}

@Module
interface InterfaceModule {
    @Provides fun provideTitle(): String
}

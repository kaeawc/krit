package fixtures.negative.slacklint

import dagger.Module
import dagger.Provides

@Module
abstract class NameModule {
    companion object {
        @Provides fun provideName(): String = "name"
    }
}

@Module
interface InterfaceModule {
    companion object {
        @Provides fun provideTitle(): String = "title"
    }
}

package fixtures.positive.slacklint

import dagger.Module
import dagger.Provides

@Module
object InitModule {
    @Provides fun provideInit() {
        println("init")
    }

    @Provides fun provideNothing(): Unit = Unit
}

package fixtures.negative.slacklint

@dagger.Module
object NameModule {
    @dagger.Provides fun provideName(): String = "name"
}

@dagger.Module
abstract class CompanionModule {
    companion object {
        @dagger.Provides fun provideTitle(): String = "title"
    }
}

// A local annotation that only shares Dagger's simple name.
annotation class Provides

class NotAModule {
    @Provides fun provideName(): String = "name"
}

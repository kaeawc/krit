package smoke

@me.tatarka.inject.annotations.Scope
annotation class Singleton

class InjectProviderSmoke {
    @me.tatarka.inject.annotations.Provides
    fun provideText(): String = "smoke"
}

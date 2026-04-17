package test

annotation class Module
annotation class Provides
annotation class Singleton

object Room {
    fun databaseBuilder(context: Context, klass: Class<AppDb>, name: String): Builder = Builder()
}

class Context

fun appContext(): Context = Context()

class Builder {
    fun build(): AppDb = AppDb()
}

class AppDb

@Module
object DbModule {
    @Provides
    @Singleton
    fun provideDb(context: Context): AppDb =
        Room.databaseBuilder(context, AppDb::class.java, "app.db").build()
}

class Holder {
    companion object {
        val db: AppDb = Room.databaseBuilder(appContext(), AppDb::class.java, "app.db").build()
    }
}

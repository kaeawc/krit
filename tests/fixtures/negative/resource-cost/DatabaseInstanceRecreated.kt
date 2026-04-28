package test

import androidx.room.Room

annotation class Module
annotation class Provides
annotation class Singleton

class Context

fun appContext(): Context = Context()

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

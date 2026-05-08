package test

import androidx.room.Room

class Context
class AppDb

object BuildConfig {
    const val DEBUG = true
}

object DbModule {
    fun provideDb(context: Context): AppDb {
        val builder = Room.databaseBuilder(context, AppDb::class.java, "app.db")
        if (BuildConfig.DEBUG) {
            builder.fallbackToDestructiveMigration()
        }
        return builder.build()
    }

    fun provideDbWithMigrations(context: Context): AppDb =
        Room.databaseBuilder(context, AppDb::class.java, "app.db").build()
}

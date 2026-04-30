package test

import androidx.room.Room

class Context
class AppDb

object DbModule {
    fun provideDb(context: Context): AppDb =
        Room.databaseBuilder(context, AppDb::class.java, "app.db")
            .fallbackToDestructiveMigration()
            .build()
}

package com.example.app.libraries

import android.content.Context
import androidx.room.Dao
import androidx.room.Database
import androidx.room.Entity
import androidx.room.PrimaryKey
import androidx.room.Query
import androidx.room.Room
import androidx.room.RoomDatabase
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import okhttp3.OkHttpClient
import okhttp3.Request
import retrofit2.Retrofit
import retrofit2.http.GET
import timber.log.Timber

class NetworkLibraryCoverage {
    private val okHttpClient = OkHttpClient.Builder().build()

    private val retrofit =
        Retrofit.Builder()
            .baseUrl("https://api.example.com/")
            .client(okHttpClient)
            .build()

    private val api = retrofit.create(UserApi::class.java)

    fun fetchRawUsers(): String {
        val request = Request.Builder()
            .url("https://api.example.com/users")
            .build()

        return try {
            okHttpClient.newCall(request).execute().use { response ->
                response.body?.string().orEmpty()
            }
        } catch (e: Exception) {
            Timber.e(e, "Failed to fetch users")
            ""
        }
    }

    fun retrofitApi(): UserApi = api
}

interface UserApi {
    @GET("users")
    suspend fun users(): List<RemoteUser>
}

data class RemoteUser(
    val id: String,
    val name: String,
)

@Entity(tableName = "cached_users")
data class CachedUser(
    @PrimaryKey val id: String,
    val name: String,
)

@Dao
interface CachedUserDao {
    @Query("SELECT * FROM cached_users")
    fun getAll(): List<CachedUser>
}

@Database(entities = [CachedUser::class], version = 1)
abstract class AppDatabase : RoomDatabase() {
    abstract fun cachedUsers(): CachedUserDao
}

class DatabaseLibraryCoverage(context: Context) {
    val database: AppDatabase =
        Room.databaseBuilder(context, AppDatabase::class.java, "playground.db")
            .fallbackToDestructiveMigration()
            .build()
}

class SyncWorker(
    context: Context,
    params: WorkerParameters,
) : CoroutineWorker(context, params) {
    override suspend fun doWork(): Result {
        Timber.d("Running playground sync")
        return Result.success()
    }
}

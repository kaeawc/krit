// Smoke: entity/DAO/database annotations with their real arguments and the builder.
package stubs

import android.content.Context
import androidx.room.ColumnInfo
import androidx.room.Dao
import androidx.room.Database
import androidx.room.Delete
import androidx.room.Embedded
import androidx.room.Entity
import androidx.room.ForeignKey
import androidx.room.Ignore
import androidx.room.Index
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.PrimaryKey
import androidx.room.Query
import androidx.room.Room
import androidx.room.RoomDatabase
import androidx.room.Transaction
import androidx.room.TypeConverter
import androidx.room.TypeConverters
import androidx.room.Update
import androidx.room.Upsert
import androidx.room.migration.Migration
import androidx.sqlite.db.SupportSQLiteDatabase
import java.util.Date
import kotlinx.coroutines.flow.Flow

data class Address(val street: String)

@Entity(tableName = "users", indices = [Index(value = ["email"], unique = true)])
data class User(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    @ColumnInfo(name = "email", index = true) val email: String,
    @Embedded(prefix = "home_") val address: Address,
    @Ignore val transient: String = "",
)

@Entity(
    tableName = "pets",
    foreignKeys = [
        ForeignKey(
            entity = User::class,
            parentColumns = ["id"],
            childColumns = ["owner_id"],
            onDelete = ForeignKey.CASCADE,
        ),
    ],
)
data class Pet(@PrimaryKey val id: Long, @ColumnInfo(name = "owner_id") val ownerId: Long)

@Dao
interface UserDao {
    @Query("SELECT * FROM users")
    fun all(): List<User>

    @Query("SELECT * FROM users WHERE id = :id")
    fun observe(id: Long): Flow<User?>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insert(user: User): Long

    @Insert
    fun insertAll(vararg users: User)

    @Update
    suspend fun update(user: User)

    @Delete
    suspend fun delete(user: User)

    @Upsert
    suspend fun upsert(user: User)

    @Transaction
    suspend fun replaceAll(users: List<User>) {
        users.forEach { insert(it) }
    }
}

class Converters {
    @TypeConverter
    fun fromTimestamp(value: Long?): Date? = value?.let { Date(it) }

    @TypeConverter
    fun toTimestamp(date: Date?): Long? = date?.time
}

@Database(entities = [User::class, Pet::class], version = 2, exportSchema = false)
@TypeConverters(Converters::class)
abstract class AppDatabase : RoomDatabase() {
    abstract fun userDao(): UserDao
}

val MIGRATION_1_2: Migration = object : Migration(1, 2) {
    override fun migrate(db: SupportSQLiteDatabase) {
        db.execSQL("ALTER TABLE users ADD COLUMN age INTEGER NOT NULL DEFAULT 0")
    }
}

fun buildDb(context: Context): AppDatabase =
    Room.databaseBuilder(context, AppDatabase::class.java, "app.db")
        .addMigrations(MIGRATION_1_2)
        .fallbackToDestructiveMigration(dropAllTables = true)
        .addCallback(object : RoomDatabase.Callback() {
            override fun onCreate(db: SupportSQLiteDatabase) {}
        })
        .build()

fun inMemory(context: Context): AppDatabase =
    Room.inMemoryDatabaseBuilder(context, AppDatabase::class.java).allowMainThreadQueries().build()

// Compiler-test source stubs; never packaged in the production artifact.
package androidx.room

import android.content.Context
import androidx.room.migration.Migration
import androidx.sqlite.db.SupportSQLiteDatabase
import kotlin.reflect.KClass

// Shapes follow Room 2.7 (Kotlin sources): constant holders are annotation
// companions, so `OnConflictStrategy.REPLACE` resolves to
// `OnConflictStrategy.Companion.REPLACE`. Java METHOD targets map to
// FUNCTION plus the property accessors; TYPE maps to CLASS.

@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.BINARY)
annotation class Dao

@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.BINARY)
annotation class Database(
    val entities: Array<KClass<*>> = [],
    val views: Array<KClass<*>> = [],
    val version: Int,
    val exportSchema: Boolean = true,
)

@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.BINARY)
annotation class Entity(
    val tableName: String = "",
    val indices: Array<Index> = [],
    val inheritSuperIndices: Boolean = false,
    val primaryKeys: Array<String> = [],
    val foreignKeys: Array<ForeignKey> = [],
    val ignoredColumns: Array<String> = [],
)

@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER, AnnotationTarget.PROPERTY_SETTER)
@Retention(AnnotationRetention.BINARY)
annotation class Query(val value: String)

@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER, AnnotationTarget.PROPERTY_SETTER)
@Retention(AnnotationRetention.BINARY)
annotation class RawQuery(val observedEntities: Array<KClass<*>> = [])

@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER, AnnotationTarget.PROPERTY_SETTER)
@Retention(AnnotationRetention.BINARY)
annotation class Transaction

@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER, AnnotationTarget.PROPERTY_SETTER)
@Retention(AnnotationRetention.BINARY)
annotation class Insert(
    val entity: KClass<*> = Any::class,
    @OnConflictStrategy val onConflict: Int = OnConflictStrategy.ABORT,
)

@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER, AnnotationTarget.PROPERTY_SETTER)
@Retention(AnnotationRetention.BINARY)
annotation class Update(
    val entity: KClass<*> = Any::class,
    @OnConflictStrategy val onConflict: Int = OnConflictStrategy.ABORT,
)

@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER, AnnotationTarget.PROPERTY_SETTER)
@Retention(AnnotationRetention.BINARY)
annotation class Delete(val entity: KClass<*> = Any::class)

@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER, AnnotationTarget.PROPERTY_SETTER)
@Retention(AnnotationRetention.BINARY)
annotation class Upsert(val entity: KClass<*> = Any::class)

@Target(AnnotationTarget.FIELD, AnnotationTarget.FUNCTION)
@Retention(AnnotationRetention.BINARY)
annotation class PrimaryKey(val autoGenerate: Boolean = false)

@Target(AnnotationTarget.FIELD, AnnotationTarget.FUNCTION)
@Retention(AnnotationRetention.BINARY)
annotation class ColumnInfo(
    val name: String = INHERIT_FIELD_NAME,
    val typeAffinity: Int = UNDEFINED,
    val index: Boolean = false,
    val collate: Int = UNSPECIFIED,
    val defaultValue: String = VALUE_UNSPECIFIED,
) {
    companion object {
        const val INHERIT_FIELD_NAME: String = "[field-name]"
        const val UNDEFINED: Int = 1
        const val TEXT: Int = 2
        const val INTEGER: Int = 3
        const val REAL: Int = 4
        const val BLOB: Int = 5
        const val UNSPECIFIED: Int = 1
        const val BINARY: Int = 2
        const val NOCASE: Int = 3
        const val VALUE_UNSPECIFIED: String = "[value-unspecified]"
    }
}

@Target(
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY_SETTER,
    AnnotationTarget.FIELD,
    AnnotationTarget.CONSTRUCTOR,
)
@Retention(AnnotationRetention.BINARY)
annotation class Ignore

@Target(AnnotationTarget.FIELD, AnnotationTarget.FUNCTION)
@Retention(AnnotationRetention.BINARY)
annotation class Embedded(val prefix: String = "")

@Target
@Retention(AnnotationRetention.BINARY)
annotation class Index(
    vararg val value: String,
    val name: String = "",
    val unique: Boolean = false,
)

@Target
@Retention(AnnotationRetention.BINARY)
annotation class ForeignKey(
    val entity: KClass<*>,
    val parentColumns: Array<String>,
    val childColumns: Array<String>,
    val onDelete: Int = NO_ACTION,
    val onUpdate: Int = NO_ACTION,
    val deferred: Boolean = false,
) {
    companion object {
        const val NO_ACTION: Int = 1
        const val RESTRICT: Int = 2
        const val SET_NULL: Int = 3
        const val SET_DEFAULT: Int = 4
        const val CASCADE: Int = 5
    }
}

@Retention(AnnotationRetention.BINARY)
annotation class OnConflictStrategy {
    companion object {
        const val NONE: Int = 0
        const val REPLACE: Int = 1

        @Deprecated("Use ABORT instead.")
        const val ROLLBACK: Int = 2
        const val ABORT: Int = 3

        @Deprecated("Use ABORT instead.")
        const val FAIL: Int = 4
        const val IGNORE: Int = 5
    }
}

@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER, AnnotationTarget.PROPERTY_SETTER)
@Retention(AnnotationRetention.BINARY)
annotation class TypeConverter

@Target(
    AnnotationTarget.CLASS,
    AnnotationTarget.FIELD,
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY_SETTER,
    AnnotationTarget.VALUE_PARAMETER,
)
@Retention(AnnotationRetention.BINARY)
annotation class TypeConverters(vararg val value: KClass<*>)

abstract class RoomDatabase {
    open val isOpen: Boolean
        get() = TODO()

    open fun close() {
        TODO()
    }

    abstract fun clearAllTables()

    open fun runInTransaction(body: Runnable) {
        TODO()
    }

    open fun <V> runInTransaction(body: java.util.concurrent.Callable<V>): V = TODO()

    abstract class Callback {
        open fun onCreate(db: SupportSQLiteDatabase) {
            TODO()
        }

        open fun onOpen(db: SupportSQLiteDatabase) {
            TODO()
        }
    }

    open class Builder<T : RoomDatabase> {
        open fun addMigrations(vararg migrations: Migration): Builder<T> = TODO()

        open fun addCallback(callback: Callback): Builder<T> = TODO()

        open fun allowMainThreadQueries(): Builder<T> = TODO()

        // The Boolean overloads of fallbackToDestructiveMigration and
        // fallbackToDestructiveMigrationOnDowngrade are final (`actual fun`
        // in the KMP source); the deprecated no-argument forms are open.
        @Deprecated(
            "Replace by overloaded version with parameter to indicate if all tables should be dropped or not.",
            ReplaceWith("fallbackToDestructiveMigration(false)"),
        )
        open fun fallbackToDestructiveMigration(): Builder<T> = TODO()

        fun fallbackToDestructiveMigration(dropAllTables: Boolean): Builder<T> = TODO()

        @Deprecated(
            "Replace by overloaded version with parameter to indicate if all tables should be dropped or not.",
            ReplaceWith("fallbackToDestructiveMigrationOnDowngrade(false)"),
        )
        open fun fallbackToDestructiveMigrationOnDowngrade(): Builder<T> = TODO()

        fun fallbackToDestructiveMigrationOnDowngrade(dropAllTables: Boolean): Builder<T> = TODO()

        @Deprecated(
            "Replace by overloaded version with parameter to indicate if all tables should be dropped or not.",
            ReplaceWith("fallbackToDestructiveMigrationFrom(false, startVersions)"),
        )
        open fun fallbackToDestructiveMigrationFrom(vararg startVersions: Int): Builder<T> = TODO()

        open fun fallbackToDestructiveMigrationFrom(dropAllTables: Boolean, vararg startVersions: Int): Builder<T> =
            TODO()

        open fun createFromAsset(databaseFilePath: String): Builder<T> = TODO()

        open fun build(): T = TODO()
    }
}

object Room {
    fun <T : RoomDatabase> databaseBuilder(context: Context, klass: Class<T>, name: String?): RoomDatabase.Builder<T> =
        TODO()

    inline fun <reified T : RoomDatabase> databaseBuilder(
        context: Context,
        name: String,
        noinline factory: () -> T = { TODO() },
    ): RoomDatabase.Builder<T> = TODO()

    fun <T : RoomDatabase> inMemoryDatabaseBuilder(context: Context, klass: Class<T>): RoomDatabase.Builder<T> =
        TODO()
}

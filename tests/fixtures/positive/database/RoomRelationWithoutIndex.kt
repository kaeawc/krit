package test

annotation class Entity(val tableName: String = "", val indices: Array<Index> = [])
annotation class Index(vararg val value: String, val name: String = "", val unique: Boolean = false)
annotation class PrimaryKey(val autoGenerate: Boolean = false)
annotation class Embedded
annotation class Relation(
    val parentColumn: String,
    val entityColumn: String,
    val entity: kotlin.reflect.KClass<*> = Any::class,
)

@Entity
data class User(
    @PrimaryKey val id: Long,
    val name: String,
)

@Entity
data class Post(
    @PrimaryKey val id: Long,
    val userId: Long,
)

data class UserWithPosts(
    @Embedded val user: User,
    @Relation(parentColumn = "id", entityColumn = "userId")
    val posts: List<Post>,
)

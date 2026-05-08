package test

annotation class Entity
annotation class PrimaryKey

@Entity
data class User(
    @PrimaryKey val id: Long,
    var name: String,
    val createdAt: Long,
)

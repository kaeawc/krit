package test

annotation class Entity
annotation class PrimaryKey

@Entity
data class User(
    @PrimaryKey val id: Long,
    val name: String,
    val createdAt: Long,
)

data class DraftUser(
    var name: String,
)

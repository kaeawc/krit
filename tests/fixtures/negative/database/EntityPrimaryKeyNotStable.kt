package test

annotation class Entity
annotation class PrimaryKey(val autoGenerate: Boolean = false)

@Entity
data class User(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    val name: String,
)

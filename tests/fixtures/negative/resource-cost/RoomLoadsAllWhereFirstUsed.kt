package fixtures.negative.resourcecost

annotation class Dao
annotation class Query(val value: String)

class User

@Dao
interface UserDao {
    @Query("SELECT * FROM users LIMIT 1")
    fun getAll(): List<User>

    @Query("SELECT * FROM users LIMIT 1")
    fun getFirst(): User
}

class PlainRepository {
    fun getAll(): List<User> = emptyList()
}

class RoomLoadsAllWhereFirstUsed(private val dao: UserDao, private val repository: PlainRepository) {
    fun getFirstUser(dao: UserDao): Any {
        return dao.getFirst()
    }

    fun getFirstFromLimitedQuery(): Any {
        return dao.getAll().first()
    }

    fun getFirstFromPlainCollection(): Any {
        return repository.getAll().first()
    }
}

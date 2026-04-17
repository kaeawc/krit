package fixtures.positive.resourcecost

class RoomLoadsAllWhereFirstUsed {
    fun getFirstUser(dao: UserDao): Any {
        return dao.getAll().first()
    }
}

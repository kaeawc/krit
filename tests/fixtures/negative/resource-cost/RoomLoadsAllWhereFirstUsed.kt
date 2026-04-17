package fixtures.negative.resourcecost

class RoomLoadsAllWhereFirstUsed {
    fun getFirstUser(dao: UserDao): Any {
        return dao.getFirst()
    }
}

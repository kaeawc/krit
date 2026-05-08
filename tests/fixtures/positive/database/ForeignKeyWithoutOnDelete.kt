package test

import kotlin.reflect.KClass

class ForeignKey(
    val entity: KClass<*>,
    val parentColumns: Array<String>,
    val childColumns: Array<String>,
    val onDelete: Int = NO_ACTION,
) {
    companion object {
        const val NO_ACTION = 0
        const val CASCADE = 1
        const val RESTRICT = 2
        const val SET_NULL = 3
    }
}

class Team

val teamForeignKey = ForeignKey(
    entity = Team::class,
    parentColumns = arrayOf("id"),
    childColumns = arrayOf("teamId"),
)

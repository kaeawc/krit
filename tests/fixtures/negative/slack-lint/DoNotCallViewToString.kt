package fixtures.negative.slacklint

import android.content.Context
import android.view.View

class Badge(val count: Int) {
    override fun toString(): String = "Badge($count)"
}

fun describe(badge: Badge, view: View): String = badge.toString() + view.id

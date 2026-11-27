package fixtures.negative.slacklint

import android.content.Context
import android.view.View

class Badge(val count: Int) {
    override fun toString(): String = "Badge($count)"
}

fun describe(badge: Badge, view: View): String = badge.toString() + view.id

class NullableView(context: Context) : View(context) {
    override fun toString(): String = "NullableView"
}

// On a nullable receiver, written through a typealias or as a type parameter's nullable bound, toString() is
// Any?.toString(), not the View's.
typealias NullableViewAlias = NullableView?

fun NullableViewAlias.aliasDescribe(): String = toString()

fun <T : NullableView?> T.boundDescribe(): String = toString()

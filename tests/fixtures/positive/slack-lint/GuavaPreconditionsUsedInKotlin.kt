package fixtures.positive.slacklint

import com.google.common.base.Preconditions

fun open(path: String) {
    Preconditions.checkArgument(path.isNotEmpty())
}

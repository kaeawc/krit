package fixtures.negative.slacklint

import com.google.common.base.Strings

fun open(path: String) {
    require(path.isNotEmpty()) { "empty path" }
    Strings.isNullOrEmpty(path)
}

package kotlin_settings

@SupportsAssignment
class Box {
    var value: String = ""

    fun assign(next: String) {
        value = next
    }
}

class Holder {
    val box = Box()
}

// Compiles only with the assignment plugin: `box` is a val.
fun rename(holder: Holder) {
    holder.box = "renamed"
}

// Compiles only with -XXLanguage:+ContextSensitiveResolutionUsingExpectedType:
// RED and GREEN resolve against the expected type, Color.
enum class Color { RED, GREEN }

fun code(color: Color): Int =
    when (color) {
        RED -> 1
        GREEN -> 2
    }

// The FIR-only finding, which krit reports only when the file compiles.
fun length(holder: Holder): Int? = holder.box.value?.length

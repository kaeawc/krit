// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 23, 37
// A `name` is a fixed token only when it is an enum entry's name or a
// declared name read through reflection. Go skips every `.name` read by its
// text, so it misses a person's or a file's name shown to the user, and it
// reports an enum's own `name` written without a receiver.
package ulimtokenname

import java.io.File
import kotlin.reflect.KProperty1

data class User(val name: String)

class Person(val name: String) {
    // Go skips `this.name` by its text; the name is user-facing text.
    fun label(): String = Text(<!UpperLowerInvariantMisuse!>this.name.uppercase()<!>)
}

enum class Level {
    Low;

    // Go reports this: `name` is the entry's constant name, a fixed token.
    fun label(): String = Text(name.lowercase())
}

// Go skips these by the `.name` text; the names are shown to the user.
fun userName(user: User): String = Text(<!UpperLowerInvariantMisuse!>user.name.uppercase()<!>)

fun nullableUserName(user: User?): String = Text(<!UpperLowerInvariantMisuse!>user?.name?.lowercase()<!>)

fun fileName(file: File): String = Text(<!UpperLowerInvariantMisuse!>file.name.uppercase()<!>)

// Fixed tokens: an enum's name (with or without a receiver) and reflective names.
fun enumName(level: Level, nullable: Level?): String = Text(level.name.lowercase()) + Text(nullable?.name?.uppercase())

// Go reports this: `name` is the receiver's enum name.
fun implicitEnumName(level: Level): String = with(level) { Text(name.uppercase()) }

fun className(value: Any): String = Text(value.javaClass.name.lowercase())

fun propertyName(property: KProperty1<User, String>): String = Text(property.name.uppercase())

fun Text(text: String?): String = text.orEmpty()

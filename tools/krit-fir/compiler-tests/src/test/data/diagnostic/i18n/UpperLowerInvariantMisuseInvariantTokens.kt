// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 42, 45x2, 48
// Receivers that hold fixed ASCII tokens (a string literal, a `.name` read, an
// enum's toString()) and arguments of valueOf-style lookups are not user-facing
// text, so they take no display locale. Text built from other values is still
// reported.
package ulimtokens

enum class Level {
    Low,
    High;

    fun label(): String = Text(toString().lowercase())

    fun thisLabel(): String = Text(this.toString().uppercase())
}

enum class Color {
    Red;

    override fun toString(): String = "Red Color"
}

fun valueOfOrNull(name: String): Level? = Level.entries.firstOrNull { it.name == name }

// Negatives.
fun literal(): String = Text("Title".lowercase()) + Text("x".uppercase())

fun nameRead(level: Level, nullable: Level?): String = Text(level.name.lowercase()) + Text(nullable?.name?.uppercase())

fun enumToString(level: Level): String = Text(level.toString().uppercase())

fun nullableEnumToString(level: Level?): String? = Text(level?.toString()?.uppercase())

fun valueOfArgument(raw: String): Level = Level.valueOf(Text(raw.uppercase()))

fun enumValueOfArgument(raw: String): Level = enumValueOf(Text(raw.trim().uppercase()))

fun valueOfOrNullArgument(raw: String): Level? = valueOfOrNull(Text(raw.lowercase()))

// Positives.
fun template(user: String): String = Text(<!UpperLowerInvariantMisuse!>"Hi $user".uppercase()<!>)

fun otherToString(value: Any, count: Int): String =
    Text(<!UpperLowerInvariantMisuse!>value.toString().uppercase()<!>) + Text(<!UpperLowerInvariantMisuse!>count.toString().lowercase()<!>)

// A lambda inside the arguments ends the valueOf context.
fun lambdaInsideValueOf(raw: String): Level = Level.valueOf(run { Text(<!UpperLowerInvariantMisuse!>raw.uppercase()<!>) })

// An enum that overrides toString() can return display text; Go skips it when
// the enum is declared in the same file.
fun overridden(color: Color): String = Text(<!UpperLowerInvariantMisuse!>color.toString().uppercase()<!>)

fun Text(text: String?): String = text.orEmpty()

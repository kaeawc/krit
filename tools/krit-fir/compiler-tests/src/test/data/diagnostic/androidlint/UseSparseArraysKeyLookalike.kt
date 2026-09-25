// RENDER_DIAGNOSTICS_FULL_TEXT
// Divergence (precision): each map below is a java.util.HashMap, but its key
// is not kotlin.Int or kotlin.Long, so the message ("Use SparseArray instead of
// HashMap<Int, ...>") is false for this code and FIR does not report it. Go
// reads the key by the last identifier of the written type, so it reports
// every one of them. The import form (`import other.Long`) is pinned in
// UseSparseArraysCrossFileTest.
package test

object Holder {
    class Long
    class Int
    class Boolean
}

// Go takes the last type_identifier `Long` and suggests LongSparseArray; the
// key is test.Holder.Long.
fun nestedLongKey(): Map<Holder.Long, String> = HashMap<Holder.Long, String>()

// Go takes `Int` and suggests SparseArray; the key is test.Holder.Int.
fun nestedIntKey(): Map<Holder.Int, String> = HashMap<Holder.Int, String>()

// A type parameter named Int shadows kotlin.Int inside the function. Go reads
// `Int` and suggests SparseArray; the key is the type parameter, which can be
// any type.
fun <Int> typeParameterKey(): Map<Int, String> = HashMap<Int, String>()

// A key of kotlin.Int still reports next to the lookalikes.
fun realKey(): Map<kotlin.Int, String> = <!UseSparseArrays!>HashMap<kotlin.Int, String>()<!>

// Divergence (message only): the key is kotlin.Int, so both report on this
// line, but the value is test.Holder.Boolean, not kotlin.Boolean. Go reads the
// value's last identifier `Boolean` and suggests SparseBooleanArray; FIR
// suggests SparseArray, which is correct. The finding is on the same line, so
// the verdict merge confirms Go's finding and keeps Go's (wrong) message. The
// `import other.Boolean` form is pinned in UseSparseArraysCrossFileTest.
fun nestedBooleanValue(): Map<kotlin.Int, Holder.Boolean> = <!UseSparseArrays!>HashMap<kotlin.Int, Holder.Boolean>()<!>

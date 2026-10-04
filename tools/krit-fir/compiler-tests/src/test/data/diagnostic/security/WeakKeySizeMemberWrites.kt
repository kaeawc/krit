// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 34, 35, 37, 45, 54, 62
// A member property's writes count only on the object the call reads it
// from, and a member val's algorithm comes from the getInstance call that
// produces its value.
package test

import javax.crypto.KeyGenerator

class Holder {
    lateinit var gen: KeyGenerator
}

// `b.gen` is the AES generator and 128 meets its minimum; the HmacSHA256
// generator was written to `a.gen`.
fun otherInstance(a: Holder, b: Holder) {
    b.gen = KeyGenerator.getInstance("AES")
    a.gen = KeyGenerator.getInstance("HmacSHA256")
    with(b) { gen.init(128) }
}

// The same through apply blocks: `b`'s generator is AES.
fun applyBlocks() {
    val b = Holder().apply { gen = KeyGenerator.getInstance("AES") }
    val a = Holder().apply { gen = KeyGenerator.getInstance("HmacSHA256") }
    with(b) { gen.init(128) }
    println(a)
}

// Weak sizes on the same objects still report.
fun sameInstance(a: Holder, b: Holder) {
    b.gen = KeyGenerator.getInstance("AES")
    a.gen = KeyGenerator.getInstance("HmacSHA256")
    with(b) { <!WeakKeySize!>gen.init(64)<!> }
    b.run { <!WeakKeySize!>gen.init(64)<!> }
    val c = Holder().apply { gen = KeyGenerator.getInstance("AES") }
    with(c) { <!WeakKeySize!>gen.init(64)<!> }
}

class Owner {
    lateinit var gen: KeyGenerator

    fun thisAssign() {
        this.gen = KeyGenerator.getInstance("AES")
        <!WeakKeySize!>gen.init(64)<!>
    }

    // Go reports: it reads the first write to a `gen` in the function, the
    // HmacSHA256 one to `other.gen`. The call initializes this.gen, the AES
    // generator, and 128 meets its minimum.
    fun implicitAssign(other: Owner) {
        other.gen = KeyGenerator.getInstance("HmacSHA256")
        gen = KeyGenerator.getInstance("AES")
        gen.init(128)
    }

    companion object {
        lateinit var shared: KeyGenerator

        fun setup() {
            shared = KeyGenerator.getInstance("AES")
            <!WeakKeySize!>shared.init(64)<!>
        }
    }
}

// The member val's value is the AES generator, the run block's last
// statement; 128 meets its minimum.
class MemberRun {
    private val gen: KeyGenerator = run {
        KeyGenerator.getInstance("HmacSHA256").init(256)
        KeyGenerator.getInstance("AES")
    }

    fun use() {
        gen.init(128)
    }
}

// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 23, 32, 38, 45, 53, 61
package test

import javax.inject.Singleton

@Singleton
class Cache {
    <!SingletonOnMutableClass!>var current: String? = null<!>
    val entries = mutableListOf<String>()
    val keys = mutableSetOf<String>()
    val values = mutableMapOf<String, String>()
    val array = arrayListOf<String>()
    val hashes = hashSetOf<String>()
    val mappings = hashMapOf<String, String>()
    val linked = linkedSetOf<String>()
    val ordered = linkedMapOf<String, String>()
    fun reset() { current = null }
}

@Singleton
class Constructors {
    <!SingletonOnMutableClass!>val entries = ArrayList<String>()<!>
    val keys = HashSet<String>()
    val mappings = HashMap<String, String>()
    val linked = LinkedHashSet<String>()
    val ordered = LinkedHashMap<String, String>()
}

@Singleton
class InternallyAssigned {
    <!SingletonOnMutableClass!>lateinit var value: String<!>
    fun reset(next: String) { this.value = next }
}

@Singleton
class PartlyConfined {
    <!SingletonOnMutableClass!>private var value = 0<!>
    @androidx.annotation.MainThread fun bump() { value++ }
    fun reset() { value = 0 }
}

@Singleton
class Multiline {
    <!SingletonOnMutableClass!>@Deprecated("use another cache")<!>
    var value = 0
    fun bump() { value++ }
}

fun localCache(): Any {
    @Singleton
    class Local {
        <!SingletonOnMutableClass!>var value = 0<!>
        fun bump() { value++ }
    }
    return Local()
}

@Singleton
class UnsynchronizedAccess {
    <!SingletonOnMutableClass!>private var value = 0<!>
    @Synchronized fun bump() { value++ }
    fun read() = value
}

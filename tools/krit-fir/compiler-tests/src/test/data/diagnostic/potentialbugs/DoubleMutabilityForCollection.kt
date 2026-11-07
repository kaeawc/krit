// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 12, 14, 16, 18, 20, 22, 24, 26, 28, 31, 33, 35, 37, 39, 41, 43, 45, 49, 55, 57, 59, 61, 65, 70, 74, 75, 79, 83, 84, 90, 93, 96, 101, 110, 119
// Positive: private (or local) `var` properties whose type is a mutable
// collection and that are never written after their declaration. Each is
// reported on the property's first line (modifier list, else `var`), the line
// the Go rule reports. Every case here is also a Go finding.
package test

class Holder {
    <!DoubleMutabilityForCollection!>private<!> var list: MutableList<String> = mutableListOf()

    <!DoubleMutabilityForCollection!>private<!> var set: MutableSet<String> = mutableSetOf()

    <!DoubleMutabilityForCollection!>private<!> var map: MutableMap<String, Int> = mutableMapOf()

    <!DoubleMutabilityForCollection!>private<!> var arrayList: ArrayList<String> = ArrayList()

    <!DoubleMutabilityForCollection!>private<!> var hashMap: HashMap<String, Int> = HashMap()

    <!DoubleMutabilityForCollection!>private<!> var hashSet: HashSet<String> = HashSet()

    <!DoubleMutabilityForCollection!>private<!> var linkedMap: LinkedHashMap<String, Int> = LinkedHashMap()

    <!DoubleMutabilityForCollection!>private<!> var linkedSet: LinkedHashSet<String> = LinkedHashSet()

    <!DoubleMutabilityForCollection!>private<!> var qualified: kotlin.collections.MutableList<String> = mutableListOf()

    <!DoubleMutabilityForCollection!>private<!> var javaQualified: java.util.ArrayList<String> = java.util.ArrayList()

    // Inferred from a factory Go lists.
    <!DoubleMutabilityForCollection!>private<!> var inferredList = mutableListOf<String>()

    <!DoubleMutabilityForCollection!>private<!> var inferredMap = hashMapOf<String, Int>()

    <!DoubleMutabilityForCollection!>private<!> var inferredArrayList = arrayListOf<String>()

    <!DoubleMutabilityForCollection!>private<!> var inferredLinkedMap = linkedMapOf<String, Int>()

    <!DoubleMutabilityForCollection!>private<!> var inferredLinkedSet = linkedSetOf<String>()

    <!DoubleMutabilityForCollection!>private<!> var inferredHashSet = hashSetOf<String>()

    <!DoubleMutabilityForCollection!>private<!> var inferredConstructor = ArrayList<String>()

    <!DoubleMutabilityForCollection!>private<!> var fullyQualifiedFactory = kotlin.collections.mutableSetOf<String>()

    // MutableCollection is not in the default mutableTypes, but the
    // initializer is a factory and the type is still a mutable collection.
    <!DoubleMutabilityForCollection!>private<!> var collection: MutableCollection<String> = mutableListOf()

    /**
     * KDoc is not part of the reported line.
     */
    <!DoubleMutabilityForCollection!>@Volatile<!>
    private var annotated: MutableList<String> = mutableListOf()

    <!DoubleMutabilityForCollection!>private<!> var private: MutableList<String> = mutableListOf()

    <!DoubleMutabilityForCollection!>private<!> lateinit var late: MutableList<String>

    <!DoubleMutabilityForCollection!>private<!> var getterBacked: MutableList<String>
        get() = mutableListOf()
        set(value) {}

    <!DoubleMutabilityForCollection!>private<!> var String.extension: MutableList<String>
        get() = mutableListOf(this)
        set(value) {}

    companion object {
        <!DoubleMutabilityForCollection!>private<!> var shared: MutableMap<String, String> = mutableMapOf()
    }

    fun local(): Int {
        <!DoubleMutabilityForCollection!>var<!> localList = mutableListOf<String>()
        <!DoubleMutabilityForCollection!>var<!> localExplicit: MutableSet<String> = mutableSetOf()
        localList.add("a")
        localExplicit.add("b")
        val lambda = {
            <!DoubleMutabilityForCollection!>var<!> inLambda = mutableListOf<Int>()
            inLambda.size
        }
        <!DoubleMutabilityForCollection!>@Suppress("CanBeVal")<!>
        var annotatedLocal = mutableListOf<String>()
        <!DoubleMutabilityForCollection!>lateinit<!> var lateLocal: MutableList<String>
        return lambda() + annotatedLocal.size + lateLocal.size
    }
}

object Registry {
    <!DoubleMutabilityForCollection!>private<!> var entries: MutableMap<String, Int> = mutableMapOf()
}

<!DoubleMutabilityForCollection!>private<!> var topLevel: MutableList<String> = mutableListOf()

fun anonymous(): Any = object {
    <!DoubleMutabilityForCollection!>private<!> var member: MutableList<String> = mutableListOf()
}

fun localClass(): Int {
    class Local {
        <!DoubleMutabilityForCollection!>private<!> var member = mutableListOf<String>()

        fun size(): Int = member.size
    }
    return Local().size()
}

enum class Kind {
    A {
        <!DoubleMutabilityForCollection!>private<!> var member = mutableListOf<String>()
    },
}

// A local function named like a factory that returns an anonymous
// java.util.HashSet subclass: the type is an anonymous object, still a mutable
// set. Go reports it by the call name.
fun anonymousSubclass(): Int {
    fun hashSetOf() = object : java.util.HashSet<String>() {}
    <!DoubleMutabilityForCollection!>var<!> s = hashSetOf()
    return s.size
}

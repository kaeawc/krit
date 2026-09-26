// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 11, 13, 15, 17, 19, 21, 23, 25, 27, 29, 32, 34, 36, 38, 40, 42, 44, 46, 50, 55, 58, 60, 62, 66, 71, 75, 76, 80, 83, 85, 95, 99, 103, 106, 109, 114, 121, 130
// Positive: `var` properties whose type is a mutable collection. Each is
// reported on the property's first line (modifier list, else `var`), the line
// the Go rule reports. Every case here is also a Go finding.
package test

class Holder {
    <!DoubleMutabilityForCollection!>var<!> list: MutableList<String> = mutableListOf()

    <!DoubleMutabilityForCollection!>var<!> set: MutableSet<String> = mutableSetOf()

    <!DoubleMutabilityForCollection!>var<!> map: MutableMap<String, Int> = mutableMapOf()

    <!DoubleMutabilityForCollection!>var<!> nullable: MutableList<String>? = null

    <!DoubleMutabilityForCollection!>var<!> arrayList: ArrayList<String> = ArrayList()

    <!DoubleMutabilityForCollection!>var<!> hashMap: HashMap<String, Int> = HashMap()

    <!DoubleMutabilityForCollection!>var<!> hashSet: HashSet<String> = HashSet()

    <!DoubleMutabilityForCollection!>var<!> linkedMap: LinkedHashMap<String, Int> = LinkedHashMap()

    <!DoubleMutabilityForCollection!>var<!> linkedSet: LinkedHashSet<String> = LinkedHashSet()

    <!DoubleMutabilityForCollection!>var<!> qualified: kotlin.collections.MutableList<String> = mutableListOf()

    <!DoubleMutabilityForCollection!>var<!> javaQualified: java.util.ArrayList<String> = java.util.ArrayList()

    // Inferred from a factory Go lists.
    <!DoubleMutabilityForCollection!>var<!> inferredList = mutableListOf<String>()

    <!DoubleMutabilityForCollection!>var<!> inferredMap = hashMapOf<String, Int>()

    <!DoubleMutabilityForCollection!>var<!> inferredArrayList = arrayListOf<String>()

    <!DoubleMutabilityForCollection!>var<!> inferredLinkedMap = linkedMapOf<String, Int>()

    <!DoubleMutabilityForCollection!>var<!> inferredLinkedSet = linkedSetOf<String>()

    <!DoubleMutabilityForCollection!>var<!> inferredHashSet = hashSetOf<String>()

    <!DoubleMutabilityForCollection!>var<!> inferredConstructor = ArrayList<String>()

    <!DoubleMutabilityForCollection!>var<!> fullyQualifiedFactory = kotlin.collections.mutableSetOf<String>()

    // MutableCollection is not in the default mutableTypes, but the
    // initializer is a factory and the type is still a mutable collection.
    <!DoubleMutabilityForCollection!>var<!> collection: MutableCollection<String> = mutableListOf()

    /**
     * KDoc is not part of the reported line.
     */
    <!DoubleMutabilityForCollection!>@Volatile<!>
    var annotated: MutableList<String> = mutableListOf()

    <!DoubleMutabilityForCollection!>private<!> var private: MutableList<String> = mutableListOf()

    <!DoubleMutabilityForCollection!>lateinit<!> var late: MutableList<String>

    <!DoubleMutabilityForCollection!>var<!> getterBacked: MutableList<String>
        get() = mutableListOf()
        set(value) {}

    <!DoubleMutabilityForCollection!>var<!> String.extension: MutableList<String>
        get() = mutableListOf(this)
        set(value) {}

    companion object {
        <!DoubleMutabilityForCollection!>var<!> shared: MutableMap<String, String> = mutableMapOf()
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
        annotatedLocal = mutableListOf()
        lateLocal = annotatedLocal
        localList = lateLocal
        localExplicit = mutableSetOf()
        return lambda()
    }
}

interface Api {
    <!DoubleMutabilityForCollection!>var<!> abstractList: MutableList<String>
}

class Impl : Api {
    <!DoubleMutabilityForCollection!>override<!> var abstractList: MutableList<String> = mutableListOf()
}

object Registry {
    <!DoubleMutabilityForCollection!>var<!> entries: MutableMap<String, Int> = mutableMapOf()
}

<!DoubleMutabilityForCollection!>var<!> topLevel: MutableList<String> = mutableListOf()

fun anonymous(): Any = object {
    <!DoubleMutabilityForCollection!>var<!> member: MutableList<String> = mutableListOf()
}

fun localClass(): Int {
    class Local {
        <!DoubleMutabilityForCollection!>var<!> member = mutableListOf<String>()
    }
    return Local().member.size
}

enum class Kind {
    A {
        <!DoubleMutabilityForCollection!>var<!> member = mutableListOf<String>()
    },
}

// A local function named like a factory that returns an anonymous
// java.util.HashSet subclass: the type is an anonymous object, still a mutable
// set. Go reports it by the call name.
fun anonymousSubclass(): Int {
    fun hashSetOf() = object : java.util.HashSet<String>() {}
    <!DoubleMutabilityForCollection!>var<!> s = hashSetOf()
    s = hashSetOf()
    return s.size
}

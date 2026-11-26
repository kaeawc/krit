package fixtures.androidlint

import androidx.lifecycle.MutableLiveData

class ProfileViewModel {
    val name = MutableLiveData<String>()
    val nickname = MutableLiveData<String?>()

    fun update(input: String?) {
        nickname.value = null
        if (input != null) {
            name.value = input
        }
        name.value = input ?: "anonymous"
    }

    // Smart casts Lint follows: a safe-call lambda, early exits, require/check and a when exit.
    fun smartCasts(a: String?, b: String?, c: String?, d: String?) {
        a?.let { name.value = a }
        if (b.isNullOrEmpty()) return
        name.value = b
        require(c != null)
        name.value = c
        when {
            d == null -> return
        }
        name.value = d
    }

    // Smart casts from a positive guard, an else branch, a Nothing exit, an elvis exit and a when subject.
    fun moreSmartCasts(a: String?, b: String?, c: String?, d: String?, e: String?, f: String?) {
        if (!a.isNullOrEmpty()) name.value = a
        if (b == null) {
            println("none")
        } else {
            name.value = b
        }
        if (c == null) error("missing")
        name.value = c
        val known = d ?: error("missing")
        name.value = d
        when (e) {
            null -> return
            else -> name.value = e
        }
        f ?: run { return }
        name.value = f + known
    }
}

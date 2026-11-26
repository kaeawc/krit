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
}

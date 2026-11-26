package fixtures.androidlint

import androidx.lifecycle.MutableLiveData

class ProfileViewModel {
    val name = MutableLiveData<String>()

    fun clear() {
        name.value = null
    }

    fun returnLater() = Unit

    // A run block that calls a function, rather than returning, does not prove input non-null.
    fun update(input: String?) {
        input ?: run { returnLater() }
        name.value = input
    }
}

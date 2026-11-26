package fixtures.androidlint

import androidx.lifecycle.MutableLiveData

class ProfileViewModel {
    val name = MutableLiveData<String>()

    fun clear() {
        name.value = null
    }
}

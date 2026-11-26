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
}

package dihygiene

import javax.inject.Singleton
annotation class Inject

class User
class Entry

@Singleton
class UserCache @Inject constructor() {
    var currentUser: User? = null
    val entries = mutableListOf<Entry>()
}

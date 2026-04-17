package test

class ContactsScreen(private val resolver: Any) {
    fun loadContacts() {
        resolver.query(ContactsContract.CommonDataKinds.Phone.CONTENT_URI, null, null, null, null)
    }
}

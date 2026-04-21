package test

class ContactsScreen(private val resolver: ContentResolver) {
    fun loadContacts() {
        resolver.query(ContactsContract.CommonDataKinds.Phone.CONTENT_URI, null, null, null, null)
    }
}

package test

class ContactsScreen(private val resolver: ContentResolver) {
    private val requestContactsPermission = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { granted ->
        if (granted) {
            resolver.query(ContactsContract.CommonDataKinds.Phone.CONTENT_URI, null, null, null, null)
        }
    }

    fun loadContacts() {
        requestContactsPermission.launch(Manifest.permission.READ_CONTACTS)
    }
}

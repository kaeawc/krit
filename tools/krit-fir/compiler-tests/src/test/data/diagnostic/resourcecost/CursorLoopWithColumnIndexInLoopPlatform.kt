// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 28, 34, 41, 49
package cursorloop.platform

import android.content.ContentResolver
import android.database.Cursor
import android.provider.ContactsContract
import android.provider.ContactsContract.Contacts
import android.provider.ContactsContract.Contacts as MemberContacts
import android.provider.ContactsContract.Contacts as VariableContacts

fun contactIds(queryRows: (Int, Int) -> Cursor?, limit: Int, offset: Int) {
    queryRows(limit, offset)?.use { rowCursor ->
        while (rowCursor.moveToNext()) {
            val id =
                rowCursor.getString(
                    rowCursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndexOrThrow<!>(
                        ContactsContract.CommonDataKinds.Contactables.CONTACT_ID
                    )
                )
        }
    }
}

fun contactKeys(resolver: ContentResolver) {
    resolver.query(Contacts.CONTENT_URI, null, null, null, null)?.use { cursor ->
        while (cursor.moveToNext()) {
            val key = cursor.getString(cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndexOrThrow<!>(Contacts.LOOKUP_KEY))
        }
    }
}

fun qualifiedContactKeys(cursor: Cursor) {
    while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>(android.provider.ContactsContract.Contacts.LOOKUP_KEY)
}

class ContactQueryTests {
    fun testContactIds(queryRows: (Int, Int) -> Cursor?) {
        queryRows(10, 0)?.use { rowCursor ->
            while (rowCursor.moveToNext()) {
                val id = rowCursor.getString(rowCursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndexOrThrow<!>(ContactsContract.CommonDataKinds.Contactables.CONTACT_ID))
            }
        }
    }

    fun testContactKeys(resolver: ContentResolver) {
        resolver.query(Contacts.CONTENT_URI, null, null, null, null)?.use { cursor ->
            while (cursor.moveToNext()) {
                val key = cursor.getString(cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndexOrThrow<!>(Contacts.LOOKUP_KEY))
            }
        }
    }
}

class VariableContactColumns(var LOOKUP_KEY: String)

fun variableContactKey(cursor: Cursor, VariableContacts: VariableContactColumns) {
    while (cursor.moveToNext()) cursor.getColumnIndex(VariableContacts.LOOKUP_KEY)
}

fun rowDependentContactKey(cursor: Cursor) {
    while (cursor.moveToNext()) cursor.getColumnIndex(Contacts.LOOKUP_KEY + cursor.getString(0))
}

fun nestedContactCursor(cursor: Cursor) {
    while (cursor.moveToNext()) {
        cursor.use { rows -> rows.getColumnIndex(Contacts.LOOKUP_KEY) }
    }
}

// FIR-only: the unrelated local name is resolved independently; Go rejects any bound path segment.
fun unrelatedSegmentBinding(cursor: Cursor, LOOKUP_KEY: String) {
    while (cursor.moveToNext()) cursor.<!CursorLoopWithColumnIndexInLoop!>getColumnIndex<!>(Contacts.LOOKUP_KEY)
}

class MemberContactReader {
    object MemberContacts { var LOOKUP_KEY = "lookup" }
    fun read(cursor: Cursor) {
        while (cursor.moveToNext()) cursor.getColumnIndex(MemberContacts.LOOKUP_KEY)
    }
}

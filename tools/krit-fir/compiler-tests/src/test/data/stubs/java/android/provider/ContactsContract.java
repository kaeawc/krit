// Compiler-test source stub; never packaged in the production artifact.
package android.provider;

import android.net.Uri;

public final class ContactsContract {
    private ContactsContract() {
        throw new RuntimeException("Stub!");
    }

    // BaseColumns and the SDK column interfaces are not modeled; their constants
    // LOOKUP_KEY (ContactsColumns) and CONTACT_ID (DataColumns) are declared directly.
    public static class Contacts {
        private Contacts() {
            throw new RuntimeException("Stub!");
        }

        public static final Uri CONTENT_URI = Uri.parse("content://com.android.contacts/contacts");
        public static final String LOOKUP_KEY = "lookup";
    }

    public static final class CommonDataKinds {
        private CommonDataKinds() {
            throw new RuntimeException("Stub!");
        }

        public static final class Contactables {
            private Contactables() {
                throw new RuntimeException("Stub!");
            }

            public static final String CONTACT_ID = "contact_id";
        }
    }
}

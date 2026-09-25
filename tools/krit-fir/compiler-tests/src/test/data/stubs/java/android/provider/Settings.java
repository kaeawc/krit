// Compiler-test source stub; never packaged in the production artifact.
package android.provider;

import android.content.ContentResolver;

// Real Global/Secure/System extend Settings.NameValueTable, not modeled.
public final class Settings {
    public static final String ACTION_SETTINGS = "android.settings.SETTINGS";

    public static final String ACTION_APPLICATION_DETAILS_SETTINGS = "android.settings.APPLICATION_DETAILS_SETTINGS";

    public Settings() {
    }

    public static final class Global {
        public static final String AIRPLANE_MODE_ON = "airplane_mode_on";

        public Global() {
        }

        public static String getString(ContentResolver resolver, String name) {
            throw new RuntimeException("Stub!");
        }

        public static int getInt(ContentResolver cr, String name, int def) {
            throw new RuntimeException("Stub!");
        }

        public static boolean putString(ContentResolver resolver, String name, String value) {
            throw new RuntimeException("Stub!");
        }
    }

    public static final class Secure {
        public static final String ANDROID_ID = "android_id";

        public Secure() {
        }

        public static String getString(ContentResolver resolver, String name) {
            throw new RuntimeException("Stub!");
        }

        public static int getInt(ContentResolver cr, String name, int def) {
            throw new RuntimeException("Stub!");
        }
    }

    public static final class System {
        public static final String SCREEN_BRIGHTNESS = "screen_brightness";

        public System() {
        }

        public static String getString(ContentResolver resolver, String name) {
            throw new RuntimeException("Stub!");
        }

        public static int getInt(ContentResolver cr, String name, int def) {
            throw new RuntimeException("Stub!");
        }
    }
}

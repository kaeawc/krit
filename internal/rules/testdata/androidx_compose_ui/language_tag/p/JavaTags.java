package p;

import androidx.compose.ui.text.intl.Locale;
import androidx.compose.ui.text.intl.LocaleList;

class JavaTags {
    static final String TAG = "fi_FI";

    void tags() {
        Locale a = new Locale("en_US");
        LocaleList b = new LocaleList("en_US,da_DK");
        Locale c = new Locale(TAG);
        Locale ok = new Locale("en-US");
        java.util.Locale platform = new java.util.Locale("en_US");
    }
}

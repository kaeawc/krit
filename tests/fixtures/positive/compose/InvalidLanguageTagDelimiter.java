package fixtures.compose;

import androidx.compose.ui.text.intl.LocaleList;

class InvalidLanguageTagDelimiter {
    LocaleList locales() {
        return new LocaleList("en_US,fr_FR");
    }
}

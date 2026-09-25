// Compiler-test source stub; never packaged in the production artifact.
package android.icu.text;

import java.util.Date;

// Real chain: DateFormat -> UFormat -> java.text.Format, not modeled.
public abstract class DateFormat {
    protected DateFormat() {
    }

    public final String format(Date date) {
        throw new RuntimeException("Stub!");
    }

    public Date parse(String text) {
        throw new RuntimeException("Stub!");
    }
}

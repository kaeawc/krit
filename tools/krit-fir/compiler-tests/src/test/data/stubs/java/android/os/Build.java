// Compiler-test source stub; never packaged in the production artifact.
package android.os;

public class Build {
    // Read from system properties at runtime in the SDK, so none of these are
    // compile-time constants: FIR must not constant-fold them. `= null` and
    // Integer.parseInt(...) keep the initializers non-constant.
    public static final String BRAND = null;

    public static final String DEVICE = null;

    public static final String FINGERPRINT = null;

    public static final String MANUFACTURER = null;

    public static final String MODEL = null;

    public Build() {
    }

    public static class VERSION {
        public static final int SDK_INT = Integer.parseInt("0");

        public static final String RELEASE = null;

        public static final String CODENAME = null;

        public VERSION() {
        }
    }

    public static class VERSION_CODES {
        public static final int BASE = 1;

        public static final int HONEYCOMB = 11;

        public static final int ICE_CREAM_SANDWICH = 14;

        public static final int JELLY_BEAN = 16;

        public static final int JELLY_BEAN_MR1 = 17;

        public static final int JELLY_BEAN_MR2 = 18;

        public static final int KITKAT = 19;

        public static final int LOLLIPOP = 21;

        public static final int LOLLIPOP_MR1 = 22;

        public static final int M = 23;

        public static final int N = 24;

        public static final int N_MR1 = 25;

        public static final int O = 26;

        public static final int O_MR1 = 27;

        public static final int P = 28;

        public static final int Q = 29;

        public static final int R = 30;

        public static final int S = 31;

        public static final int S_V2 = 32;

        public static final int TIRAMISU = 33;

        public static final int UPSIDE_DOWN_CAKE = 34;

        public static final int VANILLA_ICE_CREAM = 35;

        public VERSION_CODES() {
        }
    }
}

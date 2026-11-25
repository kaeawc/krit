package p;

import java.util.ArrayList;

public class A {
    private int count;
    private static int total;
    private static final int LIMIT = 10;
    private static final int DOUBLE = LIMIT * 2;
    private void bump() {}
    private static void reset() {}

    static class Nested {
        void x() {
            A.reset();
            int t = A.total;
            int d = DOUBLE;
        }
    }

    class Shadow {
        int count;
        void x() {
            count++;
            int bump = 0;
            bump++;
        }
    }

    class Sub extends ArrayList<String> {
        void x() { bump(); }
    }

    void anon() {
        int count = 0;
        Runnable r = new Runnable() {
            public void run() { int c = count; }
        };
        class Local {
            void y() { bump(); }
        }
    }

    enum Mode {
        FAST {
            void go() { helper(); }
        };
        private static void helper() {}
        void go() {}
    }

    void ref() {
        Runnable r = this::bump;
    }
}

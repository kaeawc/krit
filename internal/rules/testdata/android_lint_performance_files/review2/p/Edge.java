package p;

import java.util.ArrayList;

public class Edge {
    private int value;
    private int[] values;
    private int capacity() { return 4; }
    private void work() {}
    private void consume(Object o) {}

    interface I {
        default void work() {}
    }

    class Child implements I {
        void x() { work(); }
    }

    class Ctor {
        Object list = new ArrayList<Object>(capacity()) {};
    }

    class Fields {
        Object holder = new Object() { int value = 1; };
        int read() { return value; }
    }

    class Loops {
        void x() { for (int values : values) {} }
    }

    class Resources {
        void x() throws Exception {
            try (java.io.StringReader value = new java.io.StringReader("")) {
            } finally {
                consume(value);
            }
        }
    }
}

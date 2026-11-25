package p;

import java.util.ArrayList;

public class Review {
    private int value;
    private void clear() {}
    private void ensureCapacity(int n) {}
    private void work() {}
    private int helper() { return 1; }
    private void many(String... values) {}

    static class Base {
        Base(int x) {}
        Base() {}
        public void work() {}
    }

    class Child extends Base {
        void x() { work(); }
    }

    class Reads {
        void x() { int copy = value; }
    }

    class Varargs {
        void x() { many(); many("a", "b"); }
    }

    class Capacity extends ArrayList<Object> {
        void x() { ensureCapacity(8); }
    }

    void anon() {
        Base b = new Base(helper()) {};
        Object list = new ArrayList<Object>() {
            void y() { clear(); }
        };
    }
}

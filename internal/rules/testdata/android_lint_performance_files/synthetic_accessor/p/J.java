package p;

public class J {
    private int field;
    private static final int CONST = 1;
    private static int counter;
    private void helper() {}
    private J() {}
    J(int x) {}

    class Inner {
        private int innerField;
        private void innerMethod() {}
        private Inner() {}
        void use() {
            field++;
            helper();
            int c = CONST;
            counter++;
            new J();
        }
    }

    private class PrivateInner {
        void x() {}
    }

    static class Nested {
        private int nestedField;
        private void nestedMethod() {}
    }

    void outer() {
        Inner i = new Inner();
        i.innerMethod();
        int v = i.innerField;
        new PrivateInner();
        Nested n = new Nested();
        n.nestedMethod();
        Runnable r = new Runnable() {
            public void run() { helper(); }
        };
        Runnable l = () -> helper();
    }
}

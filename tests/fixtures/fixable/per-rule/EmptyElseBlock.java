package fixtures.positive.emptyblocks;

class EmptyElseBlockFixture {
    void check(int x) {
        if (x > 0) {
            doWork();
        } else { }
    }

    void doWork() {
    }
}

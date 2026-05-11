package fixtures.positive.emptyblocks;

class EmptyTryBlockFixture {
    void run() {
        try { } catch (Exception e) {
            handle(e);
        }
    }

    void handle(Exception e) {
    }
}

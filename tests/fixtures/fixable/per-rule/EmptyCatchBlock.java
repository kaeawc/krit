package fixtures.positive.emptyblocks;

class EmptyCatchBlockFixture {
    void handleError() {
        try {
            riskyOperation();
        } catch (Exception e) { }
    }

    void riskyOperation() throws Exception {
    }
}

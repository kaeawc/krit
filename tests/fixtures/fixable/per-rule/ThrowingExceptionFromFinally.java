package fixtures.positive.exceptions;

class ThrowingExceptionFromFinallyFixture {
    void run() throws Exception {
        try {
            work();
        } finally {
            throw new RuntimeException("masked");
        }
    }

    void work() {
    }
}

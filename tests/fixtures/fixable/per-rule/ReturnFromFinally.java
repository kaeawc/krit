package fixtures.positive.exceptions;

class ReturnFromFinallyFixture {
    int run() {
        try {
            return work();
        } finally {
            return 0;
        }
    }

    int work() {
        return 1;
    }
}

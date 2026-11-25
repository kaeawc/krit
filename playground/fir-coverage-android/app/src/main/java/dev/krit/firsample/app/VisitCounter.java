package dev.krit.firsample.app;

/** Hand-written Java in the same module as the Kotlin that calls it (#1626). */
public final class VisitCounter {
    private int visits;

    public VisitCounter(int visits) {
        this.visits = visits;
    }

    public int increment() {
        visits += 1;
        return visits;
    }

    public int getVisits() {
        return visits;
    }
}

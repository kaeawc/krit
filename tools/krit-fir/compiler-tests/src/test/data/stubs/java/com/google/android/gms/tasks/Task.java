// Compiler-test source stub; never packaged in the production artifact.
package com.google.android.gms.tasks;

// The listener and continuation members are not modeled.
public abstract class Task<TResult> {
    public Task() {
    }

    public abstract boolean isComplete();

    public abstract boolean isSuccessful();

    public abstract boolean isCanceled();

    public abstract TResult getResult();

    public abstract Exception getException();
}

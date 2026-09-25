// Compiler-test source stub; never packaged in the production artifact.
package android.os;

@Deprecated
public abstract class AsyncTask<Params, Progress, Result> {
    @Deprecated
    public AsyncTask() {
    }

    @Deprecated
    protected abstract Result doInBackground(Params... params);

    @Deprecated
    protected void onPreExecute() {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    protected void onPostExecute(Result result) {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    protected void onProgressUpdate(Progress... values) {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public final AsyncTask<Params, Progress, Result> execute(Params... params) {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public final boolean cancel(boolean mayInterruptIfRunning) {
        throw new RuntimeException("Stub!");
    }
}

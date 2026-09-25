// Compiler-test source stub; never packaged in the production artifact.
package android.widget;

import android.content.Context;
import android.util.AttributeSet;
import android.view.View;

public class ProgressBar extends View {
    public ProgressBar(Context context) {
        super(context);
    }

    public ProgressBar(Context context, AttributeSet attrs) {
        super(context, attrs);
    }

    public ProgressBar(Context context, AttributeSet attrs, int defStyleAttr) {
        super(context, attrs, defStyleAttr);
    }

    public synchronized int getProgress() {
        throw new RuntimeException("Stub!");
    }

    public synchronized void setProgress(int progress) {
        throw new RuntimeException("Stub!");
    }

    public synchronized int getMax() {
        throw new RuntimeException("Stub!");
    }

    public synchronized void setMax(int max) {
        throw new RuntimeException("Stub!");
    }

    public synchronized boolean isIndeterminate() {
        throw new RuntimeException("Stub!");
    }

    public synchronized void setIndeterminate(boolean indeterminate) {
        throw new RuntimeException("Stub!");
    }
}

// Compiler-test source stub; never packaged in the production artifact.
package android.widget;

import android.content.Context;
import android.util.AttributeSet;
import android.view.ViewGroup;

public class FrameLayout extends ViewGroup {
    public FrameLayout(Context context) {
        super(context);
    }

    public FrameLayout(Context context, AttributeSet attrs) {
        super(context, attrs);
    }

    public FrameLayout(Context context, AttributeSet attrs, int defStyleAttr) {
        super(context, attrs, defStyleAttr);
    }

    protected void onLayout(boolean changed, int left, int top, int right, int bottom) {
        throw new RuntimeException("Stub!");
    }
}

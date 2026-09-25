// Compiler-test source stub; never packaged in the production artifact.
package android.widget;

import android.content.Context;
import android.util.AttributeSet;
import android.view.ViewGroup;

@Deprecated
public class AbsoluteLayout extends ViewGroup {
    public AbsoluteLayout(Context context) {
        super(context);
    }

    public AbsoluteLayout(Context context, AttributeSet attrs) {
        super(context, attrs);
    }

    protected void onLayout(boolean changed, int l, int t, int r, int b) {
        throw new RuntimeException("Stub!");
    }
}

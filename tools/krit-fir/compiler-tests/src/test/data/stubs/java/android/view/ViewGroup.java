// Compiler-test source stub; never packaged in the production artifact.
package android.view;

import android.content.Context;
import android.util.AttributeSet;

public abstract class ViewGroup extends View implements ViewParent {
    public ViewGroup(Context context) {
        super(context);
    }

    public ViewGroup(Context context, AttributeSet attrs) {
        super(context, attrs);
    }

    public ViewGroup(Context context, AttributeSet attrs, int defStyleAttr) {
        super(context, attrs, defStyleAttr);
    }

    public ViewGroup(Context context, AttributeSet attrs, int defStyleAttr, int defStyleRes) {
        super(context, attrs, defStyleAttr, defStyleRes);
    }

    public int getChildCount() {
        throw new RuntimeException("Stub!");
    }

    public View getChildAt(int index) {
        throw new RuntimeException("Stub!");
    }

    protected abstract void onLayout(boolean changed, int l, int t, int r, int b);

    public void addView(View child) {
        throw new RuntimeException("Stub!");
    }

    public void addView(View child, LayoutParams params) {
        throw new RuntimeException("Stub!");
    }

    public void removeView(View view) {
        throw new RuntimeException("Stub!");
    }

    public void removeAllViews() {
        throw new RuntimeException("Stub!");
    }

    public static class LayoutParams {
        public static final int MATCH_PARENT = -1;

        public static final int WRAP_CONTENT = -2;

        public int width;

        public int height;

        public LayoutParams(int width, int height) {
        }
    }

    public static class MarginLayoutParams extends LayoutParams {
        public MarginLayoutParams(int width, int height) {
            super(width, height);
        }
    }
}

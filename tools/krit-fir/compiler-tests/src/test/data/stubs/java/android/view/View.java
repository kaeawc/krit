// Compiler-test source stub; never packaged in the production artifact.
package android.view;

import android.content.Context;
import android.graphics.Canvas;
import android.util.AttributeSet;

public class View {
    public static final int VISIBLE = 0;

    public static final int INVISIBLE = 4;

    public static final int GONE = 8;

    public View(Context context) {
    }

    public View(Context context, AttributeSet attrs) {
    }

    public View(Context context, AttributeSet attrs, int defStyleAttr) {
    }

    public View(Context context, AttributeSet attrs, int defStyleAttr, int defStyleRes) {
    }

    public static int generateViewId() {
        throw new RuntimeException("Stub!");
    }

    public final Context getContext() {
        throw new RuntimeException("Stub!");
    }

    public int getVisibility() {
        throw new RuntimeException("Stub!");
    }

    public void setVisibility(int visibility) {
        throw new RuntimeException("Stub!");
    }

    public boolean isEnabled() {
        throw new RuntimeException("Stub!");
    }

    public void setEnabled(boolean enabled) {
        throw new RuntimeException("Stub!");
    }

    public boolean isClickable() {
        throw new RuntimeException("Stub!");
    }

    public void setClickable(boolean clickable) {
        throw new RuntimeException("Stub!");
    }

    public float getAlpha() {
        throw new RuntimeException("Stub!");
    }

    public void setAlpha(float alpha) {
        throw new RuntimeException("Stub!");
    }

    public int getId() {
        throw new RuntimeException("Stub!");
    }

    public void setId(int id) {
        throw new RuntimeException("Stub!");
    }

    public Object getTag() {
        throw new RuntimeException("Stub!");
    }

    public void setTag(Object tag) {
        throw new RuntimeException("Stub!");
    }

    public CharSequence getContentDescription() {
        throw new RuntimeException("Stub!");
    }

    public void setContentDescription(CharSequence contentDescription) {
        throw new RuntimeException("Stub!");
    }

    public final int getWidth() {
        throw new RuntimeException("Stub!");
    }

    public final int getHeight() {
        throw new RuntimeException("Stub!");
    }

    public final ViewParent getParent() {
        throw new RuntimeException("Stub!");
    }

    public void setOnClickListener(OnClickListener l) {
        throw new RuntimeException("Stub!");
    }

    public void setOnLongClickListener(OnLongClickListener l) {
        throw new RuntimeException("Stub!");
    }

    public boolean performClick() {
        throw new RuntimeException("Stub!");
    }

    public final <T extends View> T findViewById(int id) {
        throw new RuntimeException("Stub!");
    }

    public boolean post(Runnable action) {
        throw new RuntimeException("Stub!");
    }

    public boolean postDelayed(Runnable action, long delayMillis) {
        throw new RuntimeException("Stub!");
    }

    public void invalidate() {
        throw new RuntimeException("Stub!");
    }

    public void requestLayout() {
        throw new RuntimeException("Stub!");
    }

    public void layout(int l, int t, int r, int b) {
        throw new RuntimeException("Stub!");
    }

    protected void onDraw(Canvas canvas) {
        throw new RuntimeException("Stub!");
    }

    protected void onMeasure(int widthMeasureSpec, int heightMeasureSpec) {
        throw new RuntimeException("Stub!");
    }

    protected void onLayout(boolean changed, int left, int top, int right, int bottom) {
        throw new RuntimeException("Stub!");
    }

    protected final void setMeasuredDimension(int measuredWidth, int measuredHeight) {
        throw new RuntimeException("Stub!");
    }

    public boolean onTouchEvent(MotionEvent event) {
        throw new RuntimeException("Stub!");
    }

    protected void onAttachedToWindow() {
        throw new RuntimeException("Stub!");
    }

    protected void onDetachedFromWindow() {
        throw new RuntimeException("Stub!");
    }

    public static interface OnClickListener {
        void onClick(View v);
    }

    public static interface OnLongClickListener {
        boolean onLongClick(View v);
    }
}

// Compiler-test source stub; never packaged in the production artifact.
package android.animation;

public class ValueAnimator extends Animator {
    public ValueAnimator() {
    }

    public static ValueAnimator ofFloat(float... values) {
        throw new RuntimeException("Stub!");
    }

    public static ValueAnimator ofInt(int... values) {
        throw new RuntimeException("Stub!");
    }

    public long getStartDelay() {
        throw new RuntimeException("Stub!");
    }

    public void setStartDelay(long startDelay) {
        throw new RuntimeException("Stub!");
    }

    public ValueAnimator setDuration(long duration) {
        throw new RuntimeException("Stub!");
    }

    public long getDuration() {
        throw new RuntimeException("Stub!");
    }

    public boolean isRunning() {
        throw new RuntimeException("Stub!");
    }

    public Object getAnimatedValue() {
        throw new RuntimeException("Stub!");
    }

    public void addUpdateListener(AnimatorUpdateListener listener) {
        throw new RuntimeException("Stub!");
    }

    public static interface AnimatorUpdateListener {
        void onAnimationUpdate(ValueAnimator animation);
    }
}

// Compiler-test source stub; never packaged in the production artifact.
package android.animation;

public abstract class Animator implements Cloneable {
    public Animator() {
    }

    public void start() {
        throw new RuntimeException("Stub!");
    }

    public void cancel() {
        throw new RuntimeException("Stub!");
    }

    public void end() {
        throw new RuntimeException("Stub!");
    }

    public abstract long getStartDelay();

    public abstract void setStartDelay(long startDelay);

    public abstract Animator setDuration(long duration);

    public abstract long getDuration();

    public abstract boolean isRunning();

    public void addListener(AnimatorListener listener) {
        throw new RuntimeException("Stub!");
    }

    public void removeAllListeners() {
        throw new RuntimeException("Stub!");
    }

    public static interface AnimatorListener {
        void onAnimationStart(Animator animation);

        void onAnimationEnd(Animator animation);

        void onAnimationCancel(Animator animation);

        void onAnimationRepeat(Animator animation);
    }
}

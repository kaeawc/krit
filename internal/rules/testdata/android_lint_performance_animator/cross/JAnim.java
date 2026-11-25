package p;

import android.animation.ObjectAnimator;

class JAnim {
    void run(JGauge j, Gauge g) {
        ObjectAnimator.ofFloat(j, "level", 0f, 1f);
        ObjectAnimator.ofFloat(g, "radius", 0f, 1f);
    }
}

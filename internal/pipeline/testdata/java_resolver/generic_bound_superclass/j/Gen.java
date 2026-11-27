package j;

import android.content.Context;
import android.widget.ImageView;

// The bound's "extends" is not the superclass: Gen is an ImageView.
public class Gen<T extends Comparable<T>> extends ImageView implements Runnable {
  public Gen(Context c) { super(c); }

  public void run() {}
}

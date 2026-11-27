package androidx.recyclerview.widget;
public class RecyclerView extends android.view.ViewGroup {
  public RecyclerView(android.content.Context c) { super(c); }
  protected void onLayout(boolean changed, int l, int t, int r, int b) {}
  public abstract static class ItemDecoration {
    public void onDraw(android.graphics.Canvas c, RecyclerView parent) {}
  }
}

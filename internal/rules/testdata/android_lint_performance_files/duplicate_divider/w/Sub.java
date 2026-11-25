package w;

import android.support.v7.widget.RecyclerView;

public class Sub {
    static class Base extends RecyclerView.ItemDecoration {}
    public static class DividerItemDecoration extends Base {
        int HORIZONTAL_LIST, VERTICAL_LIST;
    }
}

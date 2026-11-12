// Compiler-test source stub; never packaged in the production artifact.
package android.widget;

import android.view.View;
import android.view.ViewGroup;

// The ListView/Spinner adapter, unrelated to RecyclerView.Adapter.
public abstract class BaseAdapter {
    public void notifyDataSetChanged() {}

    public void notifyDataSetInvalidated() {}

    public abstract int getCount();

    public abstract Object getItem(int position);

    public abstract long getItemId(int position);

    public abstract View getView(int position, View convertView, ViewGroup parent);
}

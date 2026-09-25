// Compiler-test source stub; never packaged in the production artifact.
package android.view;

public interface MenuItem {
    int getItemId();

    CharSequence getTitle();

    MenuItem setTitle(CharSequence title);

    boolean isVisible();

    MenuItem setVisible(boolean visible);
}

package p.nodes;

import androidx.compose.ui.node.ModifierNodeElement;

public class JavaElement extends ModifierNodeElement<MyNode> {
    @Override public MyNode create() { return new MyNode(); }
    @Override public void update(MyNode node) {}
    @Override public int hashCode() { return 0; }
    @Override public boolean equals(Object other) { return false; }
}

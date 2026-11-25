package fixtures.compose;

import androidx.compose.ui.node.ModifierNodeElement;

public class ModifierNodeInspectableProperties extends ModifierNodeElement<TintNode> {
    @Override public TintNode create() { return new TintNode(); }
    @Override public void update(TintNode node) {}
    @Override public int hashCode() { return 0; }
    @Override public boolean equals(Object other) { return false; }
}

package rules

import (
	"context"
	"os"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/filefacts"
)

// manifestRoot parses the manifest file into a position-aware tree, shared by
// the rules of one run through the file-facts cache. It returns nil when the
// file cannot be read or parsed.
func manifestRoot(m *Manifest) *android.XMLNode {
	if m == nil || m.Path == "" {
		return nil
	}
	return filefacts.StringFact(fileFactsCache(), m.Path, slotManifestDOM, func() *android.XMLNode {
		data, err := os.ReadFile(m.Path)
		if err != nil {
			return nil
		}
		root, err := android.ParseXMLAST(context.Background(), data)
		if err != nil {
			return nil
		}
		return root
	})
}

// xmlAttrOf returns the named attribute of a node.
func xmlAttrOf(n *android.XMLNode, name string) (android.XMLAttribute, bool) {
	for _, a := range n.Attrs {
		if a.Name == name {
			return a, true
		}
	}
	return android.XMLAttribute{}, false
}

// xmlAttrValue returns the value of the named attribute, or "" when absent.
func xmlAttrValue(n *android.XMLNode, name string) string {
	a, _ := xmlAttrOf(n, name)
	return a.Value
}

// xmlChildrenNamed returns the direct children with the given tag.
func xmlChildrenNamed(n *android.XMLNode, tag string) []*android.XMLNode {
	var out []*android.XMLNode
	for _, c := range n.Children {
		if c.Tag == tag {
			out = append(out, c)
		}
	}
	return out
}

// manifestApplicationNode returns the <application> element of the tree.
func manifestApplicationNode(root *android.XMLNode) *android.XMLNode {
	if root == nil {
		return nil
	}
	for _, c := range root.Children {
		if c.Tag == "application" {
			return c
		}
	}
	return nil
}

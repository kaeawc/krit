package rules

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// networkSecurityConfigs calls visit with the root of every res/xml file whose root element is
// <network-security-config>, the files Lint's NetworkSecurityConfigDetector reads.
func networkSecurityConfigs(ctx *api.Context, visit func(path string, root *android.XMLNode)) {
	for _, resRoot := range resourceRootsFromIndex(ctx.ResourceIndex) {
		xmlDir := filepath.Join(resRoot, "xml")
		entries, err := os.ReadDir(xmlDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".xml") {
				continue
			}
			path := filepath.Join(xmlDir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil || !bytes.Contains(data, []byte("<network-security-config")) {
				continue
			}
			root, err := android.ParseXMLAST(context.Background(), data)
			if err != nil || root == nil || root.Tag != "network-security-config" {
				continue
			}
			visit(path, root)
		}
	}
}

// xmlNodeAttr returns the attribute of node with the given name.
func xmlNodeAttr(node *android.XMLNode, name string) (android.XMLAttribute, bool) {
	for _, attr := range node.Attrs {
		if attr.Name == name {
			return attr, true
		}
	}
	return android.XMLAttribute{}, false
}

// InsecureBaseConfigurationResourceRule mirrors Lint's InsecureBaseConfiguration: a base-config that
// sets cleartextTrafficPermitted="true".
type InsecureBaseConfigurationResourceRule struct {
	ValuesResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *InsecureBaseConfigurationResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *InsecureBaseConfigurationResourceRule) check(ctx *api.Context) {
	networkSecurityConfigs(ctx, func(path string, root *android.XMLNode) {
		seen := false
		for _, child := range root.Children {
			if child.Tag != "base-config" {
				continue
			}
			// Lint only handles the first base-config.
			if seen {
				break
			}
			seen = true
			if attr, ok := xmlNodeAttr(child, "cleartextTrafficPermitted"); ok && attr.Value == "true" {
				ctx.Emit(baseFindingAt(path, attr.Line, attr.Col, r.BaseRule, "Insecure Base Configuration"))
			}
		}
	})
}

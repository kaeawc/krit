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

// InvalidUsesTagAttributeRule ports Android Lint's InvalidUsesTagAttribute
// (AndroidAutoDetector): in an Android Auto descriptor such as
// res/xml/automotive_app_desc.xml, the name of a <uses> element under
// <automotiveApp> must be media, notification, sms or template. Like Lint, it
// looks at every XML resource with an <automotiveApp> root, not only the
// file the manifest meta-data points to.
type InvalidUsesTagAttributeRule struct {
	ValuesResourceBase
	AndroidRule
}

// Confidence reports a tier-1 (high) base confidence: the check compares the
// attribute value to a fixed list in the resource files.
func (r *InvalidUsesTagAttributeRule) Confidence() float64 { return api.ConfidenceHigh }

const invalidUsesTagMessage = "Expecting one of `media`, `notification`, `sms`, or `template` for the name attribute in uses tag"

func (r *InvalidUsesTagAttributeRule) check(ctx *api.Context) {
	for _, resRoot := range resourceRootsFromIndex(ctx.ResourceIndex) {
		dirs, err := os.ReadDir(resRoot)
		if err != nil {
			continue
		}
		for _, dir := range dirs {
			if !dir.IsDir() || dir.Name() != "xml" && !strings.HasPrefix(dir.Name(), "xml-") {
				continue
			}
			r.checkDir(ctx, filepath.Join(resRoot, dir.Name()))
		}
	}
}

func (r *InvalidUsesTagAttributeRule) checkDir(ctx *api.Context, xmlDir string) {
	entries, err := os.ReadDir(xmlDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".xml") {
			continue
		}
		path := filepath.Join(xmlDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil || !bytes.Contains(data, []byte("automotiveApp")) {
			continue
		}
		root, err := android.ParseXMLAST(context.Background(), data)
		if err != nil {
			continue
		}
		r.checkNode(ctx, path, root)
	}
}

func (r *InvalidUsesTagAttributeRule) checkNode(ctx *api.Context, path string, node *android.XMLNode) {
	if node.Tag == "automotiveApp" {
		for _, uses := range node.Children {
			if uses.Tag != "uses" {
				continue
			}
			for _, attr := range uses.Attrs {
				if attr.Name != "name" {
					continue
				}
				switch attr.Value {
				case "media", "notification", "sms", "template":
				default:
					ctx.Emit(baseFindingAt(path, attr.Line, attr.Col, r.BaseRule, invalidUsesTagMessage))
				}
			}
		}
	}
	for _, child := range node.Children {
		r.checkNode(ctx, path, child)
	}
}

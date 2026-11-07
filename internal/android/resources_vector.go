package android

import (
	"bytes"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var drawableFolderVersionRe = regexp.MustCompile(`-v(\d+)(?:-|$)`)

// recordVectorDrawable keeps the root size attributes and every pathData attribute of a drawable XML file.
func (idx *ResourceIndex) recordVectorDrawable(path string, data []byte, root *XMLNode) {
	vd := VectorDrawable{FilePath: path, RootTag: root.Tag}
	if m := drawableFolderVersionRe.FindStringSubmatch(filepath.Base(filepath.Dir(path))); m != nil {
		vd.FolderVersion, _ = strconv.Atoi(m[1])
	}
	attrOf := func(a XMLAttribute) VectorAttr {
		offset := int(a.EndByte) - 1 - len(a.Value)
		if offset < int(a.StartByte) || offset > len(data) {
			offset = int(a.StartByte)
		}
		lineStart := bytes.LastIndexByte(data[:offset], '\n') + 1
		return VectorAttr{
			Value: a.Value, Line: a.Line, Col: a.Col,
			ValueLine:   bytes.Count(data[:offset], []byte{'\n'}) + 1,
			ValueCol:    utf8.RuneCount(data[lineStart:offset]) + 1,
			ValueOffset: offset,
		}
	}
	var width, height *VectorAttr
	for _, a := range root.Attrs {
		v := attrOf(a)
		switch a.Name {
		case "android:width":
			width = &v
		case "android:height":
			height = &v
		}
	}
	if width != nil && height != nil {
		vd.Width, vd.Height = width, height
	}
	var walk func(n *XMLNode)
	walk = func(n *XMLNode) {
		for _, a := range n.Attrs {
			if a.Name == "pathData" || strings.HasSuffix(a.Name, ":pathData") {
				vd.PathData = append(vd.PathData, attrOf(a))
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	if vd.Width != nil || len(vd.PathData) > 0 {
		idx.VectorDrawables = append(idx.VectorDrawables, vd)
	}
}

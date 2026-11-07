package android

import (
	"context"
	"fmt"
	"html"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/tsxml"
)

func newXMLParser() *sitter.Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(tsxml.GetLanguage())
	return parser
}

var xmlParserPool = sync.Pool{
	New: func() any { return newXMLParser() },
}

// XMLAttribute is a position-aware XML attribute.
type XMLAttribute struct {
	Name      string
	Value     string
	Line      int
	Col       int
	StartByte uint
	EndByte   uint
}

// XMLNode is a position-aware XML element tree node.
type XMLNode struct {
	Tag       string
	Line      int
	Col       int
	StartByte uint
	EndByte   uint
	Attrs     []XMLAttribute
	Children  []*XMLNode
	Text      string
	// TextNodes holds each non-blank character data run with its position.
	TextNodes []XMLTextNode
}

// XMLTextNode is a run of character data and its 1-based position.
type XMLTextNode struct {
	Text string
	Line int
	Col  int
}

func (n *XMLNode) Attr(name string) string {
	for _, attr := range n.Attrs {
		if attr.Name == name {
			return attr.Value
		}
	}
	return ""
}

func (n *XMLNode) ChildByTag(tag string) *XMLNode {
	for _, child := range n.Children {
		if child.Tag == tag {
			return child
		}
	}
	return nil
}

func (n *XMLNode) ChildrenByTag(tag string) []*XMLNode {
	var out []*XMLNode
	for _, child := range n.Children {
		if child.Tag == tag {
			out = append(out, child)
		}
	}
	return out
}

func ParseXMLAST(ctx context.Context, data []byte) (*XMLNode, error) {
	pc := activeXMLParseCache.Load()
	if cached, ok := pc.Load(data); ok {
		return cached, nil
	}

	parser := xmlParserPool.Get().(*sitter.Parser)
	tree, reusable, err := scanner.ParseCtxPooled(ctx, parser, newXMLParser, data)
	if reusable != nil {
		defer xmlParserPool.Put(reusable)
	}
	if err != nil {
		return nil, fmt.Errorf("parse xml: %w", err)
	}
	root := tree.RootNode()
	if root == nil {
		return nil, fmt.Errorf("empty xml parse tree")
	}

	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		if child == nil || child.Type() != "element" {
			continue
		}
		built, err := buildXMLNode(child, data)
		if err != nil {
			return nil, err
		}
		_ = pc.SaveAsync(data, built)
		return built, nil
	}

	return nil, fmt.Errorf("no root xml element found")
}

func buildXMLNode(node *sitter.Node, src []byte) (*XMLNode, error) {
	if node == nil || node.Type() != "element" {
		return nil, fmt.Errorf("expected element node")
	}

	startTag := firstChildOfKind(node, "STag")
	if startTag == nil {
		startTag = firstChildOfKind(node, "EmptyElemTag")
	}
	if startTag == nil {
		return nil, fmt.Errorf("element missing start tag")
	}

	tagNameNode := firstChildOfKind(startTag, "Name")
	if tagNameNode == nil {
		return nil, fmt.Errorf("element missing tag name")
	}

	pos := startTag.StartPoint()
	out := &XMLNode{
		Tag:       tagNameNode.Content(src),
		Line:      int(pos.Row) + 1,
		Col:       int(pos.Column) + 1,
		StartByte: uint(startTag.StartByte()),
		EndByte:   uint(node.EndByte()),
	}

	for i := 0; i < int(startTag.ChildCount()); i++ {
		child := startTag.Child(i)
		if child == nil || child.Type() != "Attribute" {
			continue
		}
		out.Attrs = append(out.Attrs, buildXMLAttribute(child, src))
	}

	content := firstChildOfKind(node, "content")
	if content == nil {
		return out, nil
	}
	// A text run is character data and entity references up to the next element.
	var run strings.Builder
	runLine, runCol := 0, 0
	flush := func() {
		raw := run.String()
		run.Reset()
		trimmed := strings.TrimLeft(raw, " \t\r\n")
		if strings.TrimSpace(trimmed) == "" {
			return
		}
		line, col := runLine, runCol
		for _, r := range raw[:len(raw)-len(trimmed)] {
			if r == '\n' {
				line++
				col = 1
			} else {
				col++
			}
		}
		out.TextNodes = append(out.TextNodes, XMLTextNode{Text: trimmed, Line: line, Col: col})
	}
	for i := 0; i < int(content.ChildCount()); i++ {
		child := content.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "element" {
			flush()
			xmlChild, err := buildXMLNode(child, src)
			if err != nil {
				return nil, err
			}
			out.Children = append(out.Children, xmlChild)
			continue
		}
		switch child.Type() {
		case "CharData", "EntityRef", "CharRef", "Reference":
		default:
			continue
		}
		text := child.Content(src)
		if child.Type() == "CharData" {
			out.Text += text
		} else {
			text = html.UnescapeString(text)
		}
		if run.Len() == 0 {
			p := child.StartPoint()
			runLine, runCol = int(p.Row)+1, int(p.Column)+1
		}
		run.WriteString(text)
	}
	flush()

	return out, nil
}

func buildXMLAttribute(node *sitter.Node, src []byte) XMLAttribute {
	attr := XMLAttribute{
		StartByte: uint(node.StartByte()),
		EndByte:   uint(node.EndByte()),
	}
	pos := node.StartPoint()
	attr.Line = int(pos.Row) + 1
	attr.Col = int(pos.Column) + 1

	nameNode := firstChildOfKind(node, "Name")
	if nameNode != nil {
		attr.Name = nameNode.Content(src)
	}
	valueNode := firstChildOfKind(node, "AttValue")
	if valueNode != nil {
		attr.Value = strings.Trim(valueNode.Content(src), "\"'")
	}

	return attr
}

func firstChildOfKind(node *sitter.Node, kind string) *sitter.Node {
	if node == nil {
		return nil
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == kind {
			return child
		}
	}
	return nil
}

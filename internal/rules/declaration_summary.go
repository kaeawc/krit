package rules

import (
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/scanner"
)

type parameterSummaryFlat struct {
	idx            uint32
	name           string
	hasDefault     bool
	isFunctionType bool
	isProperty     bool
}

type functionDeclSummaryFlat struct {
	name             string
	hasOverride      bool
	hasOpen          bool
	hasAbstract      bool
	hasOperator      bool
	hasComposable    bool
	hasSubscribeLike bool
	hasEntryPoint    bool
	body             uint32
	paramsNode       uint32
	params           []parameterSummaryFlat
}

type classDeclSummaryFlat struct {
	name             string
	hasData          bool
	hasParcelizeLike bool
	classParams      []parameterSummaryFlat
}

type declSummaryKey struct {
	filePath string
	start    int
	end      int
	nodeType string
}

var declSummaryCache sync.Map
var declSummaryFlatCache sync.Map

func getFunctionDeclSummaryFlat(file *scanner.File, idx uint32) functionDeclSummaryFlat {
	if file == nil || idx == 0 {
		return functionDeclSummaryFlat{}
	}
	key := declSummaryKey{
		filePath: file.Path,
		start:    int(file.FlatStartByte(idx)),
		end:      int(file.FlatEndByte(idx)),
		nodeType: file.FlatType(idx),
	}
	if cached, ok := declSummaryFlatCache.Load(key); ok {
		if s, ok := cached.(functionDeclSummaryFlat); ok {
			return s
		}
	}

	var s functionDeclSummaryFlat
	s.name = extractIdentifierFlat(file, idx)
	s.hasOverride = file.FlatHasModifier(idx, "override")
	s.hasOpen = file.FlatHasModifier(idx, "open")
	s.hasAbstract = file.FlatHasModifier(idx, "abstract")
	s.hasOperator = file.FlatHasModifier(idx, "operator")
	s.hasComposable = flatHasAnnotationNamed(file, idx, "Composable")
	s.hasSubscribeLike = flatHasAnnotationNamed(file, idx, "Subscribe") ||
		flatHasAnnotationNamed(file, idx, "OnEvent") ||
		flatHasAnnotationNamed(file, idx, "EventHandler")
	s.hasEntryPoint = flatHasEntryPointAnnotation(file, idx)

	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "function_body":
			s.body = child
		case "function_value_parameters":
			s.paramsNode = child
			s.params = collectParameterSummariesFlat(file, child, "parameter")
		}
	}

	declSummaryFlatCache.Store(key, s)
	return s
}

func getClassDeclSummaryFlat(file *scanner.File, idx uint32) classDeclSummaryFlat {
	if file == nil || idx == 0 {
		return classDeclSummaryFlat{}
	}
	key := declSummaryKey{
		filePath: file.Path,
		start:    int(file.FlatStartByte(idx)),
		end:      int(file.FlatEndByte(idx)),
		nodeType: file.FlatType(idx),
	}
	if cached, ok := declSummaryFlatCache.Load(key); ok {
		if s, ok := cached.(classDeclSummaryFlat); ok {
			return s
		}
	}

	var s classDeclSummaryFlat
	s.name = extractIdentifierFlat(file, idx)
	s.hasData = file.FlatHasModifier(idx, "data")
	s.hasParcelizeLike = flatHasAnnotationNamed(file, idx, "Parcelize") ||
		flatHasAnnotationNamed(file, idx, "Serializable") ||
		flatHasAnnotationNamed(file, idx, "Entity") ||
		flatHasAnnotationNamed(file, idx, "DatabaseView")

	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		switch file.FlatType(child) {
		case "class_parameter":
			text := strings.TrimSpace(file.FlatNodeText(child))
			isProp := strings.HasPrefix(text, "val ") || strings.HasPrefix(text, "var ") ||
				strings.Contains(text, " val ") || strings.Contains(text, " var ")
			s.classParams = append(s.classParams, parameterSummaryFlat{
				idx:        child,
				name:       extractIdentifierFlat(file, child),
				hasDefault: paramHasDefaultFlat(file, child),
				isProperty: isProp,
			})
		case "primary_constructor":
			s.classParams = append(s.classParams, collectParameterSummariesFlat(file, child, "class_parameter")...)
		}
	}

	declSummaryFlatCache.Store(key, s)
	return s
}

func collectParameterSummariesFlat(file *scanner.File, parent uint32, targetType string) []parameterSummaryFlat {
	var out []parameterSummaryFlat
	if file == nil || parent == 0 {
		return out
	}
	for i := 0; i < file.FlatChildCount(parent); i++ {
		child := file.FlatChild(parent, i)
		if file.FlatType(child) != targetType {
			continue
		}
		isProp := false
		isFuncType := false
		if targetType == "class_parameter" {
			text := strings.TrimSpace(file.FlatNodeText(child))
			isProp = strings.HasPrefix(text, "val ") || strings.HasPrefix(text, "var ") ||
				strings.Contains(text, " val ") || strings.Contains(text, " var ")
		}
		if targetType == "parameter" || targetType == "class_parameter" {
			isFuncType = strings.Contains(file.FlatNodeText(child), "->")
		}
		out = append(out, parameterSummaryFlat{
			idx:            child,
			name:           extractIdentifierFlat(file, child),
			hasDefault:     paramHasDefaultFlat(file, child),
			isFunctionType: isFuncType,
			isProperty:     isProp,
		})
	}
	return out
}

func paramHasDefaultFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	return strings.Contains(file.FlatNodeText(idx), "=")
}

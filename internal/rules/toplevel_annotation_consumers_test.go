package rules

import (
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func topLevelAnnotationDecl(t *testing.T, src, kind, name string) (*scanner.File, uint32) {
	t.Helper()
	file := writeKotlinFile(t, src, "AnnotationImpact.kt")
	var decl uint32
	file.FlatWalkNodes(0, kind, func(idx uint32) {
		if decl == 0 && strings.Contains(file.FlatNodeText(idx), name) {
			decl = idx
		}
	})
	if decl == 0 {
		t.Fatalf("declaration %q (%s) missing", name, kind)
	}
	return file, decl
}

func topLevelAnnotationFindings(t *testing.T, ruleID, src string) []scanner.Finding {
	t.Helper()
	file := writeKotlinFile(t, src, "AnnotationImpact.kt")
	for _, rule := range api.Registry {
		if rule.ID == ruleID {
			cols := NewDispatcher([]*api.Rule{rule}, nil).Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %q missing", ruleID)
	return nil
}

func TestTopLevelAnnotationDeprecationConsumer(t *testing.T) {
	src := "package p\n@Deprecated(\"use new\")\nfun old() {}\nfun use() { old() }\n"
	file, old := topLevelAnnotationDecl(t, src, "function_declaration", "fun old")
	if info := extractDeprecatedInfoFlat(file, old); info == nil || info.message != "use new" {
		t.Errorf("deprecated declaration info = %+v, want message", info)
	}
	if got := topLevelAnnotationFindings(t, "Deprecation", src); len(got) == 0 {
		t.Error("deprecated top-level call was not reported")
	}
}

func TestTopLevelAnnotationHasAnnotationFlatConsumers(t *testing.T) {
	for _, tc := range []struct{ name, src, kind, decl, annotation string }{
		{"PublishedApi", "@PublishedApi(\"x\")\nfun api() {}\nfun next() {}\n", "function_declaration", "fun api", "PublishedApi"},
		{"Provides", "@Provides(\"x\")\nfun provide() = 1\nfun next() {}\n", "function_declaration", "fun provide", "Provides"},
		{"Module", "@Module(\"x\")\nclass AppModule\nfun next() {}\n", "class_declaration", "class AppModule", "Module"},
		{"Inject", "@Inject(\"x\")\nclass Service\nfun next() {}\n", "class_declaration", "class Service", "Inject"},
		{"Dao", "@Dao(\"x\")\nclass SampleDao\nfun next() {}\n", "class_declaration", "class SampleDao", "Dao"},
		{"MainThread", "@MainThread(\"x\")\nfun work() {}\nfun next() {}\n", "function_declaration", "fun work", "MainThread"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, decl := topLevelAnnotationDecl(t, tc.src, tc.kind, tc.decl)
			if !hasAnnotationFlat(file, decl, tc.annotation) {
				t.Errorf("hasAnnotationFlat lost @%s", tc.annotation)
			}
		})
	}
}

func TestTopLevelAnnotationPublishedApiRuleConsumer(t *testing.T) {
	src := "@PublishedApi(\"abi\")\nfun api() {}\nfun next() {}\n"
	for _, finding := range topLevelAnnotationFindings(t, "LibraryEntitiesShouldNotBePublic", src) {
		if finding.Line == 2 {
			t.Errorf("@PublishedApi function should be excluded, got %+v", finding)
		}
	}
}

func TestTopLevelAnnotationRoomDaoRuleConsumer(t *testing.T) {
	findings := topLevelAnnotationFindings(t, "DaoNotInterface", "@Dao(\"db\")\nclass AppDao\nfun next() {}\n")
	if len(findings) == 0 {
		t.Error("@Dao class should be reported")
	}
}

func TestTopLevelAnnotationComposeConsumers(t *testing.T) {
	file, fn := topLevelAnnotationDecl(t, "@Preview(\"phone\")\nfun render() {}\nfun next() {}\n", "function_declaration", "fun render")
	if !composeFunctionIsPreviewOrSampleFlat(file, fn) {
		t.Error("Compose preview classifier lost @Preview")
	}
	if !flatHasAnnotationNamed(file, fn, "Preview") {
		t.Error("Compose annotation lookup lost @Preview")
	}
	if !flatHasCustomPreviewAnnotation(file, fn, customPreviewConfig{Wildcard: true}) {
		t.Error("custom preview lookup lost @Preview")
	}
}

func TestTopLevelAnnotationExactNameConsumer(t *testing.T) {
	file, fn := topLevelAnnotationDecl(t, "@Composable(\"ui\")\nfun render() {}\nfun next() {}\n", "function_declaration", "fun render")
	if !hasAnnotationNamed(file, fn, "Composable") {
		t.Error("exact-name declaration annotation lookup lost @Composable")
	}
}

func TestTopLevelAnnotationAndroidAnnotationConsumer(t *testing.T) {
	file, fn := topLevelAnnotationDecl(t, "@WorkerThread(\"io\")\nfun work() {}\nfun next() {}\n", "function_declaration", "fun work")
	if !androidHasAnnotationFlat(file, fn, "WorkerThread") {
		t.Error("Android annotation lookup lost @WorkerThread")
	}
}

func TestTopLevelAnnotationOpenForTestingConsumer(t *testing.T) {
	file, fn := topLevelAnnotationDecl(t, "@OpenForTesting(\"reason\")\nfun work() {}\nfun next() {}\n", "function_declaration", "fun work")
	if !openForTestingDeclarationHasAnnotation(file, fn, nil) {
		t.Error("open-for-testing lookup lost @OpenForTesting")
	}
}

func TestTopLevelAnnotationComposeRuleConsumer(t *testing.T) {
	findings := topLevelAnnotationFindings(t, "ComposePreviewAnnotationMissing", "@Composable(\"ui\")\nfun HomePreview() {}\nfun next() {}\n")
	if len(findings) == 0 {
		t.Error("@Composable preview function without @Preview should be reported")
	}
}

func TestTopLevelAnnotationDIConsumer(t *testing.T) {
	findings := topLevelAnnotationFindings(t, "BindsMismatchedArity", "@Binds(\"x\")\nfun bind() {}\nfun next() {}\n")
	if len(findings) == 0 {
		t.Error("@Binds function with zero parameters should be reported")
	}
}

func TestTopLevelAnnotationOptInConsumer(t *testing.T) {
	findings := topLevelAnnotationFindings(t, "OptInWithoutJustification", "@OptIn(Foo::class)\nfun experimental() {}\nfun next() {}\n")
	if len(findings) == 0 {
		t.Error("@OptIn without KDoc should be reported")
	}
}

func TestTopLevelAnnotationDirectNodeConsumer(t *testing.T) {
	findings := topLevelAnnotationFindings(t, "ForbiddenOptIn", "@OptIn(Foo::class)\nfun experimental() {}\nfun next() {}\n")
	if len(findings) == 0 {
		t.Error("annotation-node rule should still see orphaned @OptIn")
	}
}

func TestTopLevelAnnotationJvmNameEntryPointConsumer(t *testing.T) {
	file, fn := topLevelAnnotationDecl(t, "@JvmName(\"javaF\")\nfun f() {}\nfun next() {}\n", "function_declaration", "fun f")
	if !flatHasEntryPointAnnotation(file, fn) {
		t.Error("unused declaration entry-point classifier lost @JvmName")
	}
}

func TestTopLevelAnnotationRequiresApiConsumer(t *testing.T) {
	file, fn := topLevelAnnotationDecl(t, "@RequiresApi(26)\nfun f() {}\nfun next() {}\n", "function_declaration", "fun f")
	if level := requiresAPILevelForDeclFlat(file, fn); level != 26 {
		t.Errorf("RequiresApi level = %d, want 26", level)
	}
}

func TestTopLevelAnnotationOrphanStatementFindings(t *testing.T) {
	src := "@Deprecated(\"x\")\nfun f() {}\nfun next() {}\n"
	for _, ruleID := range []string{"DoubleNegativeExpression", "UnusedUnaryOperator", "HiltEntryPointOnNonInterface"} {
		t.Run(ruleID, func(t *testing.T) {
			if got := topLevelAnnotationFindings(t, ruleID, src); len(got) != 0 {
				t.Errorf("orphan annotation produced false positive: %+v", got)
			}
		})
	}
}

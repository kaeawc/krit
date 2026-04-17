package oracle

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func testdataPath(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "testdata", name)
}

// ---------------------------------------------------------------------------
// Load tests
// ---------------------------------------------------------------------------

func TestLoad_ValidJSON(t *testing.T) {
	o, err := Load(testdataPath("sample_oracle.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.raw.Version != 1 {
		t.Errorf("expected version 1, got %d", o.raw.Version)
	}
	if o.raw.KotlinVersion != "2.1.0" {
		t.Errorf("expected kotlinVersion 2.1.0, got %s", o.raw.KotlinVersion)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	_, err := Load(testdataPath("invalid_oracle.json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoad_BadVersion(t *testing.T) {
	_, err := Load(testdataPath("bad_version.json"))
	if err == nil {
		t.Error("expected error for version 0")
	}
}

func TestLoad_EmptyOracle(t *testing.T) {
	o, err := Load(testdataPath("empty_oracle.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(o.classByFQN) != 0 {
		t.Errorf("expected 0 classes, got %d", len(o.classByFQN))
	}
	if o.LookupClass("anything") != nil {
		t.Error("expected nil for empty oracle")
	}
}

func TestLoad_TempFileWithBadJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(path, []byte(`{"version": 1, "kotlinVersion": "2.0", "files": {}, "dependencies": {`), 0644)
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for truncated JSON")
	}
}

// ---------------------------------------------------------------------------
// Oracle lookup tests
// ---------------------------------------------------------------------------

func TestLookupClass_ByFQN(t *testing.T) {
	o := loadSample(t)
	info := o.LookupClass("android.app.Application")
	if info == nil {
		t.Fatal("expected to find android.app.Application")
	}
	if info.Name != "Application" {
		t.Errorf("expected simple name Application, got %q", info.Name)
	}
	if info.FQN != "android.app.Application" {
		t.Errorf("expected FQN, got %q", info.FQN)
	}
	if !info.IsOpen {
		t.Error("expected Application to be open")
	}
	if info.Kind != "class" {
		t.Errorf("expected kind class, got %q", info.Kind)
	}
}

func TestLookupClass_BySimpleName(t *testing.T) {
	o := loadSample(t)
	info := o.LookupClass("Response")
	if info == nil {
		t.Fatal("expected to find Response by simple name")
	}
	if info.FQN != "retrofit2.Response" {
		t.Errorf("expected retrofit2.Response, got %q", info.FQN)
	}
}

func TestLookupClass_NotFound(t *testing.T) {
	o := loadSample(t)
	if o.LookupClass("com.nonexistent.Type") != nil {
		t.Error("expected nil for unknown type")
	}
}

func TestLookupClass_Members(t *testing.T) {
	o := loadSample(t)
	info := o.LookupClass("retrofit2.Response")
	if info == nil {
		t.Fatal("expected Response")
	}
	if len(info.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(info.Members))
	}
	// Check body() member
	var body *typeinfer.MemberInfo
	for i := range info.Members {
		if info.Members[i].Name == "body" {
			body = &info.Members[i]
		}
	}
	if body == nil {
		t.Fatal("expected body member")
	}
	if body.Kind != "function" {
		t.Errorf("expected function kind, got %q", body.Kind)
	}
}

func TestLookupClass_AbstractFlag(t *testing.T) {
	o := loadSample(t)
	info := o.LookupClass("android.content.Context")
	if info == nil {
		t.Fatal("expected Context")
	}
	if !info.IsAbstract {
		t.Error("expected Context to be abstract")
	}
}

func TestLookupClass_SourceDeclarations(t *testing.T) {
	o := loadSample(t)
	info := o.LookupClass("com.example.App")
	if info == nil {
		t.Fatal("expected source declaration com.example.App")
	}
	if len(info.Members) != 1 {
		t.Errorf("expected 1 member, got %d", len(info.Members))
	}
	if info.Members[0].IsOverride != true {
		t.Error("expected onCreate to be override")
	}
}

func TestLookupClass_SealedClass(t *testing.T) {
	o := loadSample(t)
	info := o.LookupClass("com.example.NetworkResult")
	if info == nil {
		t.Fatal("expected NetworkResult")
	}
	if !info.IsSealed {
		t.Error("expected isSealed=true")
	}
	if info.Kind != "sealed class" {
		t.Errorf("expected sealed class kind, got %q", info.Kind)
	}
}

func TestLookupClass_DataClass(t *testing.T) {
	o := loadSample(t)
	info := o.LookupClass("com.example.NetworkResult.Success")
	if info == nil {
		t.Fatal("expected Success")
	}
	if !info.IsData {
		t.Error("expected isData=true")
	}
}

// ---------------------------------------------------------------------------
// Sealed variants
// ---------------------------------------------------------------------------

func TestSealedVariants_ByFQN(t *testing.T) {
	o := loadSample(t)
	variants := o.LookupSealedVariants("com.example.NetworkResult")
	if len(variants) != 2 {
		t.Fatalf("expected 2 sealed variants, got %d: %v", len(variants), variants)
	}
}

func TestSealedVariants_BySimpleName(t *testing.T) {
	o := loadSample(t)
	variants := o.LookupSealedVariants("NetworkResult")
	if len(variants) != 2 {
		t.Fatalf("expected 2 sealed variants by simple name, got %d", len(variants))
	}
}

func TestSealedVariants_NotFound(t *testing.T) {
	o := loadSample(t)
	variants := o.LookupSealedVariants("nonexistent.Type")
	if len(variants) != 0 {
		t.Errorf("expected 0 variants, got %d", len(variants))
	}
}

// ---------------------------------------------------------------------------
// Enum entries
// ---------------------------------------------------------------------------

func TestEnumEntries_Loaded(t *testing.T) {
	o, err := Load(testdataPath("enum_oracle.json"))
	if err != nil {
		t.Fatal(err)
	}
	entries := o.LookupEnumEntries("Color")
	if len(entries) != 3 {
		t.Fatalf("expected 3 enum entries, got %d: %v", len(entries), entries)
	}
}

func TestEnumEntries_ByFQN(t *testing.T) {
	o, err := Load(testdataPath("enum_oracle.json"))
	if err != nil {
		t.Fatal(err)
	}
	entries := o.LookupEnumEntries("com.example.Color")
	if len(entries) != 3 {
		t.Fatalf("expected 3 enum entries by FQN, got %d", len(entries))
	}
}

func TestEnumEntries_NotFound(t *testing.T) {
	o := loadSample(t)
	entries := o.LookupEnumEntries("nonexistent")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

// ---------------------------------------------------------------------------
// Subtype checks
// ---------------------------------------------------------------------------

func TestIsSubtype_Direct(t *testing.T) {
	o := loadSample(t)
	if !o.IsSubtype("java.io.IOException", "java.lang.Exception") {
		t.Error("IOException should be a subtype of Exception")
	}
}

func TestIsSubtype_Transitive(t *testing.T) {
	o := loadSample(t)
	if !o.IsSubtype("java.io.IOException", "java.lang.Throwable") {
		t.Error("IOException should be a transitive subtype of Throwable")
	}
}

func TestIsSubtype_DeepChain(t *testing.T) {
	o := loadSample(t)
	// Application → ContextWrapper → Context → Any
	if !o.IsSubtype("android.app.Application", "android.content.Context") {
		t.Error("Application should be a transitive subtype of Context")
	}
	if !o.IsSubtype("android.app.Application", "kotlin.Any") {
		t.Error("Application should be a transitive subtype of Any")
	}
}

func TestIsSubtype_Self(t *testing.T) {
	o := loadSample(t)
	if !o.IsSubtype("java.io.IOException", "java.io.IOException") {
		t.Error("type should be subtype of itself")
	}
}

func TestIsSubtype_NotSubtype(t *testing.T) {
	o := loadSample(t)
	if o.IsSubtype("java.lang.Exception", "java.io.IOException") {
		t.Error("Exception should NOT be a subtype of IOException")
	}
}

func TestIsSubtype_UnknownType(t *testing.T) {
	o := loadSample(t)
	if o.IsSubtype("nonexistent.Type", "java.lang.Exception") {
		t.Error("unknown type should not be subtype of anything")
	}
}

// ---------------------------------------------------------------------------
// Function lookups
// ---------------------------------------------------------------------------

func TestLookupFunction_MemberFunction(t *testing.T) {
	o := loadSample(t)
	rt := o.LookupFunction("Application.onCreate")
	if rt == nil {
		t.Fatal("expected to find Application.onCreate")
	}
	if rt.Name != "Unit" {
		t.Errorf("expected Unit return type, got %q", rt.Name)
	}
}

func TestLookupFunction_ByFQN(t *testing.T) {
	o := loadSample(t)
	rt := o.LookupFunction("android.app.Application.onCreate")
	if rt == nil {
		t.Fatal("expected to find by FQN key")
	}
}

func TestLookupFunction_NotFound(t *testing.T) {
	o := loadSample(t)
	if o.LookupFunction("nonexistent.method") != nil {
		t.Error("expected nil for unknown function")
	}
}

func TestLookupFunction_NullableReturn(t *testing.T) {
	o := loadSample(t)
	rt := o.LookupFunction("Response.body")
	if rt == nil {
		t.Fatal("expected to find Response.body")
	}
	if !rt.Nullable {
		t.Error("expected body() to be nullable")
	}
}

// ---------------------------------------------------------------------------
// Dependencies accessor
// ---------------------------------------------------------------------------

func TestDependencies_Count(t *testing.T) {
	o := loadSample(t)
	deps := o.Dependencies()
	if len(deps) == 0 {
		t.Error("expected non-empty dependencies")
	}
	if _, ok := deps["retrofit2.Response"]; !ok {
		t.Error("expected retrofit2.Response in dependencies")
	}
}

// ---------------------------------------------------------------------------
// Expression lookup tests
// ---------------------------------------------------------------------------

func TestLookupExpression_Found(t *testing.T) {
	o := loadSample(t)
	rt := o.LookupExpression("src/main/kotlin/com/example/App.kt", 10, 5)
	if rt == nil {
		t.Fatal("expected expression type at 10:5")
	}
	if rt.Name != "String" {
		t.Errorf("expected String, got %q", rt.Name)
	}
	if rt.FQN != "kotlin.String" {
		t.Errorf("expected kotlin.String FQN, got %q", rt.FQN)
	}
	if rt.Nullable {
		t.Error("expected non-nullable")
	}
}

func TestLookupExpression_Nullable(t *testing.T) {
	o := loadSample(t)
	rt := o.LookupExpression("src/main/kotlin/com/example/App.kt", 15, 3)
	if rt == nil {
		t.Fatal("expected expression type at 15:3")
	}
	if !rt.Nullable {
		t.Error("expected nullable")
	}
	if rt.FQN != "android.app.Application" {
		t.Errorf("expected android.app.Application, got %q", rt.FQN)
	}
}

func TestLookupExpression_Primitive(t *testing.T) {
	o := loadSample(t)
	rt := o.LookupExpression("src/main/kotlin/com/example/App.kt", 12, 9)
	if rt == nil {
		t.Fatal("expected expression type at 12:9")
	}
	if rt.Name != "Int" {
		t.Errorf("expected Int, got %q", rt.Name)
	}
	if rt.Kind != typeinfer.TypePrimitive {
		t.Errorf("expected TypePrimitive, got %d", rt.Kind)
	}
}

func TestLookupExpression_WrongPosition(t *testing.T) {
	o := loadSample(t)
	rt := o.LookupExpression("src/main/kotlin/com/example/App.kt", 99, 99)
	if rt != nil {
		t.Error("expected nil for position without expression")
	}
}

func TestLookupExpression_WrongFile(t *testing.T) {
	o := loadSample(t)
	rt := o.LookupExpression("nonexistent.kt", 10, 5)
	if rt != nil {
		t.Error("expected nil for unknown file")
	}
}

func TestLookupExpression_FileWithoutExpressions(t *testing.T) {
	o, err := Load(testdataPath("empty_oracle.json"))
	if err != nil {
		t.Fatal(err)
	}
	rt := o.LookupExpression("any.kt", 1, 1)
	if rt != nil {
		t.Error("expected nil for oracle with no expressions")
	}
}

// ---------------------------------------------------------------------------
// Annotation lookup tests
// ---------------------------------------------------------------------------

func TestLookupAnnotations_MemberBySimpleName(t *testing.T) {
	o := loadSample(t)
	anns := o.LookupAnnotations("App.onCreate")
	if len(anns) != 1 {
		t.Fatalf("expected 1 annotation, got %d: %v", len(anns), anns)
	}
	if anns[0] != "androidx.annotation.CallSuper" {
		t.Errorf("expected CallSuper annotation, got %q", anns[0])
	}
}

func TestLookupAnnotations_MemberByFQN(t *testing.T) {
	o := loadSample(t)
	anns := o.LookupAnnotations("com.example.App.onCreate")
	if len(anns) != 1 {
		t.Fatalf("expected 1 annotation by FQN, got %d", len(anns))
	}
}

func TestLookupAnnotations_ClassLevel(t *testing.T) {
	o := loadSample(t)
	anns := o.LookupAnnotations("App")
	if len(anns) != 1 {
		t.Fatalf("expected 1 class annotation, got %d: %v", len(anns), anns)
	}
	if anns[0] != "javax.inject.Singleton" {
		t.Errorf("expected Singleton annotation, got %q", anns[0])
	}
}

func TestLookupAnnotations_ClassLevelByFQN(t *testing.T) {
	o := loadSample(t)
	anns := o.LookupAnnotations("com.example.App")
	if len(anns) != 1 {
		t.Fatalf("expected 1 class annotation by FQN, got %d", len(anns))
	}
}

func TestLookupAnnotations_NotFound(t *testing.T) {
	o := loadSample(t)
	anns := o.LookupAnnotations("nonexistent.method")
	if len(anns) != 0 {
		t.Errorf("expected 0 annotations, got %d", len(anns))
	}
}

// ---------------------------------------------------------------------------
// Call target lookup tests
// ---------------------------------------------------------------------------

func TestLookupCallTarget_Found(t *testing.T) {
	o := loadSample(t)
	ct := o.LookupCallTarget("src/main/kotlin/com/example/App.kt", 10, 5)
	if ct != "kotlin.text.lowercase" {
		t.Errorf("expected kotlin.text.lowercase, got %q", ct)
	}
}

func TestLookupCallTarget_NoTarget(t *testing.T) {
	o := loadSample(t)
	ct := o.LookupCallTarget("src/main/kotlin/com/example/App.kt", 12, 9)
	if ct != "" {
		t.Errorf("expected empty call target, got %q", ct)
	}
}

func TestLookupCallTarget_WrongFile(t *testing.T) {
	o := loadSample(t)
	ct := o.LookupCallTarget("nonexistent.kt", 10, 5)
	if ct != "" {
		t.Errorf("expected empty call target for unknown file, got %q", ct)
	}
}

// ---------------------------------------------------------------------------
// Diagnostic lookup tests
// ---------------------------------------------------------------------------

func TestLookupDiagnostics_Found(t *testing.T) {
	o, err := Load(testdataPath("diagnostics_oracle.json"))
	if err != nil {
		t.Fatal(err)
	}
	diags := o.LookupDiagnostics("src/main/kotlin/com/example/Unreachable.kt")
	if len(diags) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(diags))
	}
	if diags[0].FactoryName != "UNREACHABLE_CODE" {
		t.Errorf("expected UNREACHABLE_CODE, got %q", diags[0].FactoryName)
	}
	if diags[0].Severity != "WARNING" {
		t.Errorf("expected WARNING severity, got %q", diags[0].Severity)
	}
	if diags[0].Line != 5 || diags[0].Col != 9 {
		t.Errorf("expected line 5, col 9, got %d:%d", diags[0].Line, diags[0].Col)
	}
	if diags[1].FactoryName != "USELESS_ELVIS" {
		t.Errorf("expected USELESS_ELVIS, got %q", diags[1].FactoryName)
	}
	if diags[1].Line != 10 || diags[1].Col != 20 {
		t.Errorf("expected line 10, col 20, got %d:%d", diags[1].Line, diags[1].Col)
	}
}

func TestLookupDiagnostics_NoDiagnostics(t *testing.T) {
	o, err := Load(testdataPath("diagnostics_oracle.json"))
	if err != nil {
		t.Fatal(err)
	}
	diags := o.LookupDiagnostics("src/main/kotlin/com/example/Clean.kt")
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics for clean file, got %d", len(diags))
	}
}

func TestLookupDiagnostics_UnknownFile(t *testing.T) {
	o, err := Load(testdataPath("diagnostics_oracle.json"))
	if err != nil {
		t.Fatal(err)
	}
	diags := o.LookupDiagnostics("nonexistent.kt")
	if diags != nil {
		t.Errorf("expected nil for unknown file, got %v", diags)
	}
}

func TestLookupDiagnostics_EmptyOracle(t *testing.T) {
	o, err := Load(testdataPath("empty_oracle.json"))
	if err != nil {
		t.Fatal(err)
	}
	diags := o.LookupDiagnostics("any.kt")
	if diags != nil {
		t.Errorf("expected nil for empty oracle, got %v", diags)
	}
}

// ---------------------------------------------------------------------------
// FakeOracle tests
// ---------------------------------------------------------------------------

func TestFakeOracle_LookupClass(t *testing.T) {
	fake := NewFakeOracle()
	fake.Classes["Foo"] = &typeinfer.ClassInfo{Name: "Foo", FQN: "com.example.Foo"}

	info := fake.LookupClass("Foo")
	if info == nil || info.FQN != "com.example.Foo" {
		t.Error("expected FakeOracle to return configured class")
	}
	if fake.LookupClass("Bar") != nil {
		t.Error("expected nil for unconfigured class")
	}
}

func TestFakeOracle_SealedVariants(t *testing.T) {
	fake := NewFakeOracle()
	fake.Sealed["Result"] = []string{"Success", "Failure"}

	variants := fake.LookupSealedVariants("Result")
	if len(variants) != 2 {
		t.Errorf("expected 2 variants, got %d", len(variants))
	}
}

func TestFakeOracle_EnumEntries(t *testing.T) {
	fake := NewFakeOracle()
	fake.Enums["Color"] = []string{"RED", "GREEN", "BLUE"}

	entries := fake.LookupEnumEntries("Color")
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
}

func TestFakeOracle_IsSubtype(t *testing.T) {
	fake := NewFakeOracle()
	fake.Subtypes["Child"] = []string{"Parent", "GrandParent"}

	if !fake.IsSubtype("Child", "Parent") {
		t.Error("expected Child to be subtype of Parent")
	}
	if !fake.IsSubtype("Child", "Child") {
		t.Error("expected self-subtype")
	}
	if fake.IsSubtype("Parent", "Child") {
		t.Error("expected Parent NOT subtype of Child")
	}
}

func TestFakeOracle_LookupFunction(t *testing.T) {
	fake := NewFakeOracle()
	fake.Functions["Foo.bar"] = &typeinfer.ResolvedType{Name: "String", FQN: "kotlin.String"}

	rt := fake.LookupFunction("Foo.bar")
	if rt == nil || rt.Name != "String" {
		t.Error("expected String return type")
	}
}

func TestFakeOracle_LookupExpression(t *testing.T) {
	fake := NewFakeOracle()
	fake.Expressions["test.kt"] = map[string]*typeinfer.ResolvedType{
		"5:10": {Name: "String", FQN: "kotlin.String", Kind: typeinfer.TypeClass},
	}

	rt := fake.LookupExpression("test.kt", 5, 10)
	if rt == nil || rt.Name != "String" {
		t.Error("expected FakeOracle to return configured expression type")
	}
	if fake.LookupExpression("test.kt", 1, 1) != nil {
		t.Error("expected nil for unconfigured position")
	}
	if fake.LookupExpression("other.kt", 5, 10) != nil {
		t.Error("expected nil for unconfigured file")
	}
}

func TestFakeOracle_LookupAnnotations(t *testing.T) {
	fake := NewFakeOracle()
	fake.Annotations["Foo.bar"] = []string{"org.jetbrains.annotations.NotNull"}

	anns := fake.LookupAnnotations("Foo.bar")
	if len(anns) != 1 || anns[0] != "org.jetbrains.annotations.NotNull" {
		t.Errorf("expected configured annotations, got %v", anns)
	}
	if len(fake.LookupAnnotations("unknown")) != 0 {
		t.Error("expected nil for unconfigured key")
	}
}

func TestFakeOracle_LookupCallTarget(t *testing.T) {
	fake := NewFakeOracle()
	fake.CallTargets["test.kt"] = map[string]string{
		"5:10": "kotlin.collections.listOf",
	}

	ct := fake.LookupCallTarget("test.kt", 5, 10)
	if ct != "kotlin.collections.listOf" {
		t.Errorf("expected configured call target, got %q", ct)
	}
	if fake.LookupCallTarget("test.kt", 1, 1) != "" {
		t.Error("expected empty for unconfigured position")
	}
	if fake.LookupCallTarget("other.kt", 5, 10) != "" {
		t.Error("expected empty for unconfigured file")
	}
}

func TestFakeOracle_LookupDiagnostics(t *testing.T) {
	fake := NewFakeOracle()
	fake.Diagnostics["test.kt"] = []OracleDiagnostic{
		{FactoryName: "UNREACHABLE_CODE", Severity: "WARNING", Message: "Unreachable code", Line: 5, Col: 9},
	}

	diags := fake.LookupDiagnostics("test.kt")
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].FactoryName != "UNREACHABLE_CODE" {
		t.Errorf("expected UNREACHABLE_CODE, got %q", diags[0].FactoryName)
	}
	if len(fake.LookupDiagnostics("other.kt")) != 0 {
		t.Error("expected empty for unconfigured file")
	}
}

func TestFakeOracle_Dependencies(t *testing.T) {
	fake := NewFakeOracle()
	fake.Deps["com.example.Dep"] = &OracleClass{FQN: "com.example.Dep"}

	deps := fake.Dependencies()
	if len(deps) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(deps))
	}
}

// ---------------------------------------------------------------------------
// Composite Resolver tests
// ---------------------------------------------------------------------------

// fakeTypeResolver is a minimal TypeResolver for composite tests.
type fakeTypeResolver struct {
	nodeResult    *typeinfer.ResolvedType
	nameResult    *typeinfer.ResolvedType
	importResult  string
	nullResult    *bool
	classResult   *typeinfer.ClassInfo
	sealedResult  []string
	enumResult    []string
	annotResult   string
	subtypeResult bool
	indexed       bool
}

func (f *fakeTypeResolver) ResolveFlatNode(_ uint32, _ *scanner.File) *typeinfer.ResolvedType {
	if f.nodeResult != nil {
		return f.nodeResult
	}
	return typeinfer.UnknownType()
}
func (f *fakeTypeResolver) ResolveByNameFlat(_ string, _ uint32, _ *scanner.File) *typeinfer.ResolvedType {
	return f.nameResult
}
func (f *fakeTypeResolver) ResolveImport(_ string, _ *scanner.File) string { return f.importResult }
func (f *fakeTypeResolver) IsNullableFlat(_ uint32, _ *scanner.File) *bool { return f.nullResult }
func (f *fakeTypeResolver) ClassHierarchy(_ string) *typeinfer.ClassInfo   { return f.classResult }
func (f *fakeTypeResolver) SealedVariants(_ string) []string               { return f.sealedResult }
func (f *fakeTypeResolver) EnumEntries(_ string) []string                  { return f.enumResult }
func (f *fakeTypeResolver) AnnotationValueFlat(_ uint32, _ *scanner.File, _, _ string) string {
	return f.annotResult
}
func (f *fakeTypeResolver) IsExceptionSubtype(_, _ string) bool { return f.subtypeResult }
func (f *fakeTypeResolver) IndexFilesParallel(_ []*scanner.File, _ int) {
	f.indexed = true
}

func TestComposite_ClassHierarchy_OracleFirst(t *testing.T) {
	oracle := NewFakeOracle()
	oracle.Classes["Dep"] = &typeinfer.ClassInfo{Name: "Dep", FQN: "lib.Dep"}

	fallback := &fakeTypeResolver{
		classResult: &typeinfer.ClassInfo{Name: "Fallback", FQN: "src.Fallback"},
	}
	composite := NewCompositeResolver(oracle, fallback)

	// Oracle type should win
	info := composite.ClassHierarchy("Dep")
	if info == nil || info.FQN != "lib.Dep" {
		t.Error("expected oracle to provide Dep")
	}

	// Unknown type falls back
	info = composite.ClassHierarchy("Unknown")
	if info == nil || info.FQN != "src.Fallback" {
		t.Error("expected fallback for unknown type")
	}
}

func TestComposite_SealedVariants_OracleFirst(t *testing.T) {
	oracle := NewFakeOracle()
	oracle.Sealed["Result"] = []string{"A", "B"}

	fallback := &fakeTypeResolver{sealedResult: []string{"C"}}
	composite := NewCompositeResolver(oracle, fallback)

	variants := composite.SealedVariants("Result")
	if len(variants) != 2 || variants[0] != "A" {
		t.Errorf("expected oracle variants [A, B], got %v", variants)
	}

	// Fallback for unknown
	variants = composite.SealedVariants("Other")
	if len(variants) != 1 || variants[0] != "C" {
		t.Errorf("expected fallback variant [C], got %v", variants)
	}
}

func TestComposite_EnumEntries_OracleFirst(t *testing.T) {
	oracle := NewFakeOracle()
	oracle.Enums["Color"] = []string{"R", "G", "B"}

	fallback := &fakeTypeResolver{enumResult: []string{"X"}}
	composite := NewCompositeResolver(oracle, fallback)

	entries := composite.EnumEntries("Color")
	if len(entries) != 3 {
		t.Errorf("expected oracle entries, got %v", entries)
	}

	entries = composite.EnumEntries("Other")
	if len(entries) != 1 {
		t.Errorf("expected fallback entry, got %v", entries)
	}
}

func TestComposite_IsExceptionSubtype_OracleFirst(t *testing.T) {
	oracle := NewFakeOracle()
	oracle.Subtypes["MyException"] = []string{"RuntimeException"}

	fallback := &fakeTypeResolver{subtypeResult: false}
	composite := NewCompositeResolver(oracle, fallback)

	if !composite.IsExceptionSubtype("MyException", "RuntimeException") {
		t.Error("expected oracle to resolve subtype")
	}
}

func TestComposite_IsExceptionSubtype_FallbackWhenOracleMisses(t *testing.T) {
	oracle := NewFakeOracle()
	fallback := &fakeTypeResolver{subtypeResult: true}
	composite := NewCompositeResolver(oracle, fallback)

	if !composite.IsExceptionSubtype("A", "B") {
		t.Error("expected fallback to resolve subtype when oracle misses")
	}
}

func TestComposite_ResolveByNameFlat_OracleWhenFallbackNil(t *testing.T) {
	oracle := NewFakeOracle()
	oracle.Classes["DepType"] = &typeinfer.ClassInfo{Name: "DepType", FQN: "lib.DepType"}

	fallback := &fakeTypeResolver{nameResult: nil}
	composite := NewCompositeResolver(oracle, fallback)

	result := composite.ResolveByNameFlat("DepType", 1, nil)
	if result == nil || result.FQN != "lib.DepType" {
		t.Error("expected oracle to provide flat type when fallback returns nil")
	}
}

func TestComposite_ResolveImport_FallbackFirst(t *testing.T) {
	oracle := NewFakeOracle()
	oracle.Classes["Foo"] = &typeinfer.ClassInfo{Name: "Foo", FQN: "oracle.Foo"}

	fallback := &fakeTypeResolver{importResult: "src.Foo"}
	composite := NewCompositeResolver(oracle, fallback)

	result := composite.ResolveImport("Foo", nil)
	if result != "src.Foo" {
		t.Errorf("expected fallback import, got %q", result)
	}
}

func TestComposite_ResolveImport_OracleWhenFallbackEmpty(t *testing.T) {
	oracle := NewFakeOracle()
	oracle.Classes["Dep"] = &typeinfer.ClassInfo{Name: "Dep", FQN: "lib.Dep"}

	fallback := &fakeTypeResolver{importResult: ""}
	composite := NewCompositeResolver(oracle, fallback)

	result := composite.ResolveImport("Dep", nil)
	if result != "lib.Dep" {
		t.Errorf("expected oracle import, got %q", result)
	}
}

func TestComposite_AnnotationValueFlat_AlwaysFallback(t *testing.T) {
	oracle := NewFakeOracle()
	fallback := &fakeTypeResolver{annotResult: "26"}
	composite := NewCompositeResolver(oracle, fallback)

	result := composite.AnnotationValueFlat(1, nil, "RequiresApi", "value")
	if result != "26" {
		t.Errorf("expected fallback flat annotation value, got %q", result)
	}
}

func TestComposite_IndexFilesParallel_Delegates(t *testing.T) {
	oracle := NewFakeOracle()
	fallback := &fakeTypeResolver{}
	composite := NewCompositeResolver(oracle, fallback)

	composite.IndexFilesParallel(nil, 4)
	if !fallback.indexed {
		t.Error("expected IndexFilesParallel to delegate to fallback")
	}
}

// ---------------------------------------------------------------------------
// makeResolvedType tests
// ---------------------------------------------------------------------------

func TestMakeResolvedType_Primitive(t *testing.T) {
	rt := makeResolvedType("kotlin.Int", false)
	if rt.Kind != typeinfer.TypePrimitive {
		t.Errorf("expected TypePrimitive, got %d", rt.Kind)
	}
	if rt.Name != "Int" {
		t.Errorf("expected simple name Int, got %q", rt.Name)
	}
}

func TestMakeResolvedType_Nullable(t *testing.T) {
	rt := makeResolvedType("kotlin.String", true)
	if !rt.Nullable {
		t.Error("expected nullable")
	}
	if rt.Kind != typeinfer.TypeNullable {
		t.Errorf("expected TypeNullable, got %d", rt.Kind)
	}
}

func TestMakeResolvedType_Unit(t *testing.T) {
	rt := makeResolvedType("kotlin.Unit", false)
	if rt.Kind != typeinfer.TypeUnit {
		t.Errorf("expected TypeUnit, got %d", rt.Kind)
	}
}

func TestMakeResolvedType_Nothing(t *testing.T) {
	rt := makeResolvedType("kotlin.Nothing", false)
	if rt.Kind != typeinfer.TypeNothing {
		t.Errorf("expected TypeNothing, got %d", rt.Kind)
	}
}

func TestMakeResolvedType_Class(t *testing.T) {
	rt := makeResolvedType("com.example.Foo", false)
	if rt.Kind != typeinfer.TypeClass {
		t.Errorf("expected TypeClass, got %d", rt.Kind)
	}
	if rt.FQN != "com.example.Foo" {
		t.Errorf("expected FQN preserved, got %q", rt.FQN)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func loadSample(t *testing.T) *Oracle {
	t.Helper()
	o, err := Load(testdataPath("sample_oracle.json"))
	if err != nil {
		t.Fatal(err)
	}
	return o
}

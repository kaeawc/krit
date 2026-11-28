package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// gradlePackagePrefix is the package root of Gradle's public API.
const gradlePackagePrefix = "org.gradle."

// gradleManagedWildcardTypes maps the simple names of Gradle types that mark
// a class as Gradle-managed to their FQNs, so a wildcard import of the
// package (`import org.gradle.api.model.*`) proves the name is Gradle's.
// Explicit imports and qualified names need no entry: any org.gradle type
// counts there.
var gradleManagedWildcardTypes = map[string]string{
	// Services Gradle injects into @Inject constructors.
	"ObjectFactory":               "org.gradle.api.model.ObjectFactory",
	"ProviderFactory":             "org.gradle.api.provider.ProviderFactory",
	"ProjectLayout":               "org.gradle.api.file.ProjectLayout",
	"FileSystemOperations":        "org.gradle.api.file.FileSystemOperations",
	"ArchiveOperations":           "org.gradle.api.file.ArchiveOperations",
	"ExecOperations":              "org.gradle.process.ExecOperations",
	"WorkerExecutor":              "org.gradle.workers.WorkerExecutor",
	"JavaToolchainService":        "org.gradle.jvm.toolchain.JavaToolchainService",
	"BuildEventsListenerRegistry": "org.gradle.build.event.BuildEventsListenerRegistry",
	"ToolingModelBuilderRegistry": "org.gradle.tooling.provider.model.ToolingModelBuilderRegistry",
	// Supertypes of classes Gradle instantiates.
	"DefaultTask":     "org.gradle.api.DefaultTask",
	"Plugin":          "org.gradle.api.Plugin",
	"SourceTask":      "org.gradle.api.tasks.SourceTask",
	"Exec":            "org.gradle.api.tasks.Exec",
	"JavaExec":        "org.gradle.api.tasks.JavaExec",
	"WorkAction":      "org.gradle.workers.WorkAction",
	"BuildService":    "org.gradle.api.services.BuildService",
	"ValueSource":     "org.gradle.api.provider.ValueSource",
	"TransformAction": "org.gradle.api.artifacts.transform.TransformAction",
	// Managed property types Gradle implements on abstract members.
	"Property":                   "org.gradle.api.provider.Property",
	"ListProperty":               "org.gradle.api.provider.ListProperty",
	"SetProperty":                "org.gradle.api.provider.SetProperty",
	"MapProperty":                "org.gradle.api.provider.MapProperty",
	"RegularFileProperty":        "org.gradle.api.file.RegularFileProperty",
	"DirectoryProperty":          "org.gradle.api.file.DirectoryProperty",
	"ConfigurableFileCollection": "org.gradle.api.file.ConfigurableFileCollection",
}

// injectAbstractClassIsGradleManaged reports whether the abstract class at
// cls is one Gradle instantiates itself (a task, extension, build service,
// work action...): Gradle decorates the class, implements its abstract
// members and injects its services into the @Inject constructor, so the
// binding is reachable. The evidence is a constructor parameter, a direct
// supertype or an abstract property whose type resolves to Gradle's API.
func injectAbstractClassIsGradleManaged(file *scanner.File, cls, ctor uint32) bool {
	// The grammar puts class_parameter nodes directly under
	// primary_constructor or inside a class_parameters wrapper.
	gradleParam := false
	file.FlatWalkNodes(ctor, "class_parameter", func(p uint32) {
		if !gradleParam && gradleTypeNode(file, p) {
			gradleParam = true
		}
	})
	if gradleParam {
		return true
	}
	for _, super := range androidDirectSupertypesFlat(file, cls) {
		if gradleTypeName(file, super.name) {
			return true
		}
	}
	body, ok := file.FlatFindChild(cls, "class_body")
	if !ok {
		return false
	}
	for member := file.FlatFirstChild(body); member != 0; member = file.FlatNextSib(member) {
		if file.FlatType(member) != "property_declaration" || !file.FlatHasModifier(member, "abstract") {
			continue
		}
		if decl, ok := file.FlatFindChild(member, "variable_declaration"); ok && gradleTypeNode(file, decl) {
			return true
		}
	}
	return false
}

// gradleTypeNode reports whether the declared type of a class_parameter or
// variable_declaration (nullable or not) resolves to Gradle's API.
func gradleTypeNode(file *scanner.File, owner uint32) bool {
	typ, ok := file.FlatFindChild(owner, "user_type")
	if !ok {
		nullable, found := file.FlatFindChild(owner, "nullable_type")
		if !found {
			return false
		}
		if typ, ok = file.FlatFindChild(nullable, "user_type"); !ok {
			return false
		}
	}
	return gradleTypeName(file, androidUserTypeNameFlat(file, typ))
}

// gradleTypeName reports whether a written type name resolves to Gradle's
// API: qualified under org.gradle, explicitly imported from it, or a known
// Gradle type whose package the file imports with a wildcard and that no
// declaration in the file shadows.
func gradleTypeName(file *scanner.File, name string) bool {
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, gradlePackagePrefix) {
		return true
	}
	first, _, _ := strings.Cut(name, ".")
	facts := fileFactsCache().Imports(file)
	if fqn, ok := facts.Aliases[first]; ok {
		return strings.HasPrefix(fqn, gradlePackagePrefix)
	}
	if first != name {
		return false
	}
	fqn, known := gradleManagedWildcardTypes[name]
	if !known {
		return false
	}
	// Only a wildcard import brings the simple name in: an explicit import
	// of the same FQN under an alias does not.
	pkg := fqn[:strings.LastIndex(fqn, ".")]
	_, wildcard := facts.Wildcards[pkg+".*"]
	return wildcard && !fileDeclaresType(file, name)
}

// fileDeclaresType reports whether the file declares a class, object or
// type alias with the given simple name, which shadows wildcard imports.
func fileDeclaresType(file *scanner.File, name string) bool {
	found := false
	for _, kind := range []string{"class_declaration", "object_declaration", "type_alias"} {
		file.FlatWalkNodes(0, kind, func(idx uint32) {
			if !found && extractIdentifierFlat(file, idx) == name {
				found = true
			}
		})
		if found {
			return true
		}
	}
	return false
}

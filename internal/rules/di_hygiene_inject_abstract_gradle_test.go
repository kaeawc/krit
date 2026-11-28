package rules_test

import "testing"

// Gradle instantiates abstract tasks, extensions, build services and work
// actions itself (ObjectFactory.newInstance, task registration), injecting
// Gradle services into the @Inject constructor and implementing the abstract
// members, so those classes are reachable bindings.
func TestInjectOnAbstractClassSkipsGradleManagedTypes(t *testing.T) {
	cases := map[string]string{
		"task with an ExecOperations parameter": `
import javax.inject.Inject
import org.gradle.api.DefaultTask
import org.gradle.process.ExecOperations

abstract class FormatTask @Inject constructor(
    private val execOps: ExecOperations,
) : DefaultTask()
`,
		"extension with an ObjectFactory parameter": `
import javax.inject.Inject
import org.gradle.api.model.ObjectFactory

abstract class MyExtension @Inject constructor(objects: ObjectFactory)
`,
		"Gradle service through a wildcard import": `
import javax.inject.Inject
import org.gradle.api.model.*

abstract class MyExtension @Inject constructor(objects: ObjectFactory)
`,
		"fully qualified Gradle service": `
import javax.inject.Inject

abstract class MyExtension @Inject constructor(objects: org.gradle.api.model.ObjectFactory)
`,
		"Gradle supertype without parameters": `
import javax.inject.Inject
import org.gradle.workers.WorkAction
import org.gradle.workers.WorkParameters

abstract class Compile @Inject constructor() : WorkAction<WorkParameters.None>
`,
		"aliased Gradle import": `
import javax.inject.Inject
import org.gradle.api.model.ObjectFactory as GradleObjects

abstract class MyExtension @Inject constructor(objects: GradleObjects)
`,
		"nullable Gradle service": `
import javax.inject.Inject
import org.gradle.api.provider.ProviderFactory

abstract class MyExtension @Inject constructor(providers: ProviderFactory?)
`,
		"abstract Gradle managed property": `
import javax.inject.Inject
import org.gradle.api.provider.Property

abstract class Report @Inject constructor() {
    abstract val enabled: Property<Boolean>
}
`,
	}
	for name, code := range cases {
		t.Run(name, func(t *testing.T) {
			if findings := runRuleByName(t, "InjectOnAbstractClass", code); len(findings) != 0 {
				t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
			}
		})
	}
}

func TestInjectOnAbstractClassStillFlagsGradleLookalikes(t *testing.T) {
	cases := map[string]string{
		"local ObjectFactory class": `
import javax.inject.Inject

class ObjectFactory

abstract class Base @Inject constructor(objects: ObjectFactory)
`,
		"ObjectFactory from a non-Gradle wildcard": `
import javax.inject.Inject
import com.example.model.*

abstract class Base @Inject constructor(objects: ObjectFactory)
`,
		"local class shadows a Gradle wildcard": `
import javax.inject.Inject
import org.gradle.api.model.*

class ObjectFactory

abstract class Base @Inject constructor(objects: ObjectFactory)
`,
		"ObjectFactory imported from elsewhere": `
import javax.inject.Inject
import com.example.ObjectFactory

abstract class Base @Inject constructor(objects: ObjectFactory)
`,
		"Gradle FQN imported only under an alias": `
import javax.inject.Inject
import org.gradle.api.model.ObjectFactory as GradleObjects
import com.example.*

abstract class Base @Inject constructor(objects: ObjectFactory)
`,
		"abstract property of a non-Gradle type": `
import javax.inject.Inject

class Dep

abstract class Base @Inject constructor() {
    abstract val dep: Dep
}
`,
		"Gradle type only in a function body": `
import javax.inject.Inject
import org.gradle.api.model.ObjectFactory

class Dep

abstract class Base @Inject constructor(dep: Dep) {
    fun make(objects: ObjectFactory) = objects
}
`,
	}
	for name, code := range cases {
		t.Run(name, func(t *testing.T) {
			if findings := runRuleByName(t, "InjectOnAbstractClass", code); len(findings) != 1 {
				t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
			}
		})
	}
}

package rules

import (
	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
)

// profileOrDefaultMinSdk is the project-wide minSdk of rules that do not resolve it
// per file: the Gradle library profile's (a version catalog or build file),
// else the build's default for files that declare none (krit --min-sdk, set
// by the Bazel rules), else 0 for unknown.
func profileOrDefaultMinSdk(facts *librarymodel.Facts) int {
	if facts != nil && facts.Profile.MinSdkVersion > 0 {
		return facts.Profile.MinSdkVersion
	}
	return android.DefaultSDKLevels().MinSdk
}

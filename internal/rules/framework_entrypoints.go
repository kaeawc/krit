package rules

import "strings"

func isLikelyFrameworkEntryTypeName(name string) bool {
	xmlInstantiatedSuffixes := []string{
		"Fragment", "Activity", "Service", "Receiver", "Provider",
		"View", "Layout", "Transition", "Animator", "Interpolator",
		"Runner", "Application", "ContentProvider",
	}
	for _, suffix := range xmlInstantiatedSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

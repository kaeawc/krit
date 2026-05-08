package rules

// AndroidDataDependency describes which Android project data a rule needs.
type AndroidDataDependency uint32

const (
	AndroidDepNone     AndroidDataDependency = 0
	AndroidDepManifest AndroidDataDependency = 1 << iota
	AndroidDepLayout
	AndroidDepIcons
	AndroidDepGradle
	AndroidDepValuesStrings
	AndroidDepValuesDimensions
	AndroidDepValuesPlurals
	AndroidDepValuesArrays
	AndroidDepValuesExtraText
)

const AndroidDepValues = AndroidDepValuesStrings | AndroidDepValuesDimensions | AndroidDepValuesPlurals | AndroidDepValuesArrays | AndroidDepValuesExtraText
const AndroidDepResources = AndroidDepValues

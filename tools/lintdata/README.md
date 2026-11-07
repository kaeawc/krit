# Lint data generators

Android Lint keeps several tables in its jars and in the SDK. The rules in
`internal/rules` use generated copies so that krit reports what Lint does without
loading Lint at run time. Each generated file starts with `Code generated ... DO NOT EDIT`.

| Script | Output | Source of the data |
| --- | --- | --- |
| `regen-locale.sh` | `internal/rules/locale_data_gen.go` | `LocaleManager` in sdk-common: language and region names, relevant regions, alpha-3 to alpha-2 codes |
| `regen-tables.sh` | `internal/rules/lint_tables_gen.go` | `SystemPermissionsDetector`: protected permissions and the last API level at which apps could hold them |
| `regen-api-tables.sh` | `internal/rules/lint_api_tables_gen.go` | `ApiLookup` over the SDK's `api-versions.xml`: the API level of each `android.R.attr` attribute |

Run a script from anywhere; it needs a JDK and the Lint artifacts in the Gradle cache.
`LINT_VERSION` picks the Lint version (default 32.4.1). `regen-api-tables.sh` also takes the path of an
`api-versions.xml` and otherwise uses the newest platform under `$ANDROID_HOME`. It resolves the Lint
command-line classpath with `lint-classpath.sh`, which uses the Gradle wrapper in `tools/krit-fir` (it needs
network access the first time) or the file named by `LINT_CLASSPATH_FILE`.

After regenerating, run `go test ./internal/rules` and review the diff: a new Lint or SDK release changes
messages and levels the rules report.

Other rule behavior was ported by reading detector bytecode (`javap -c -p -constants`) and checking it against
the Lint command line on small synthetic projects. Gradle projects cannot be compared that way because the
command line refuses them.

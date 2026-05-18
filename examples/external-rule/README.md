# External Rule Example

Copy-paste starter for authoring a custom Krit rule. Builds a rule jar
that the host Krit binary loads via `--custom-rule-jars` (or, in a
Gradle build, the `kritCustomRules` resolvable configuration on the host
plugin — see [docs/external-rules.md](../../docs/external-rules.md#5-wire-the-jar-into-the-consumer)
for the variant-resolution rationale and
[krit-custom-rule-plugin](../../krit-custom-rule-plugin/README.md) for
the rule-authoring DSL).

## Layout

```
build.gradle.kts                    applies dev.jasonpearson.krit.custom
src/main/kotlin/...NoPrintlnRule.kt the rule implementation
src/main/resources/META-INF/services/dev.jasonpearson.krit.api.KritRule
                                     ServiceLoader registration
samples/src/main/kotlin/com/example/positive/Greeter.kt        triggers
samples/src/main/kotlin/com/example/negative/SilentGreeter.kt  does not
```

## Build the rule jar

```sh
./gradlew kritRuleJar
# → build/libs/external-rule-krit-rules.jar
```

## Use the rule jar

```sh
krit --daemon \
     --custom-rule-jars build/libs/external-rule-krit-rules.jar \
     -f json \
     samples/
```

`krit --list-rules --custom-rule-jars …` prints the rule with a leading
`P ` (plugin) marker so you can confirm load-time wiring before running
analysis.

## Prerequisites

- JDK 21+
- `dev.jasonpearson.krit:krit-rule-api` resolvable from a configured
  Maven repository. The example pins `mavenLocal()` in
  `settings.gradle.kts`; CI publishes the matching snapshot via
  `./gradlew -p tools/krit-rule-api publishToMavenLocal -PkritVersion=<v>`
  before invoking `:external-rule:kritRuleJar`.
- The Krit binary must be able to find `krit-types.jar`. Released
  binaries auto-download it; for dev builds set `KRIT_TYPES_JAR` or run
  `./gradlew -p tools/krit-types shadowJar`.

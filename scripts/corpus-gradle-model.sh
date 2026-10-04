#!/usr/bin/env bash
set -euo pipefail

fail() { echo "error: $*" >&2; exit 1; }

if [[ $# -ne 1 && $# -ne 3 ]]; then
  fail "usage: $0 <corpus-dir> [--java-home PATH]"
fi
corpus=$1
if [[ $# -eq 3 ]]; then
  [[ $2 == --java-home ]] || fail "unknown option: $2"
  [[ -x $3/bin/java ]] || fail "JAVA_HOME does not contain an executable bin/java: $3"
  export JAVA_HOME=$3
  export PATH="$JAVA_HOME/bin:$PATH"
fi
[[ -d $corpus ]] || fail "corpus directory does not exist: $corpus"
corpus=$(cd "$corpus" && pwd -P)
[[ -x $corpus/gradlew ]] || fail "missing executable corpus gradlew: $corpus/gradlew"
repo=$(cd "$(dirname "$0")/.." && pwd -P)
builder=$repo/krit-settings-plugin/gradlew
[[ -x $builder ]] || fail "missing Krit Gradle wrapper: $builder"

# The plugin has no wrapper. The settings-plugin wrapper uses the checked-in
# composite includeBuild("../krit-gradle-plugin") and builds that exact source.
# Its main source has no external runtime dependencies: Kotlin DSL and Groovy
# JSON classes come from the corpus Gradle distribution.
echo "Building local krit-gradle-plugin jar..." >&2
"$builder" -p "$repo/krit-settings-plugin" :krit-gradle-plugin:clean :krit-gradle-plugin:jar --no-daemon --no-watch-fs || fail "krit-gradle-plugin jar build failed"
shopt -s nullglob
jars=("$repo"/krit-gradle-plugin/build/libs/*.jar)
[[ ${#jars[@]} -eq 1 ]] || fail "expected one krit-gradle-plugin jar; found ${#jars[@]}"
plugin_jar=${jars[0]}

init_dir=$(mktemp -d "${TMPDIR:-/tmp}/krit-corpus-init.XXXXXX")
init_script=$init_dir/init.gradle.kts
trap 'rm -r -- "$init_dir"' EXIT
cat > "$init_script" <<'KOTLIN'
import dev.jasonpearson.krit.gradle.KritPlugin
import org.gradle.api.Action
import org.gradle.api.initialization.Settings

initscript {
    dependencies {
        classpath(files(System.getenv("KRIT_GRADLE_PLUGIN_JAR")))
    }
}

// Mirrors KritPlugin.LANGUAGE_PLUGIN_IDS and the settings plugin's beforeProject hook.
gradle.settingsEvaluated(Action<Settings> {
    // Some playgrounds already apply the settings plugin, which owns the
    // same beforeProject hook. Applying through both classloaders duplicates
    // its extension even though the class names match.
    if (!pluginManager.hasPlugin("dev.jasonpearson.krit.settings")) {
        gradle.lifecycle.beforeProject {
            val project = this
            listOf("org.jetbrains.kotlin.jvm", "com.android.application", "com.android.library").forEach { id ->
                project.pluginManager.withPlugin(id) {
                    if (project.extensions.findByName("krit") == null) {
                        project.plugins.apply(KritPlugin::class.java)
                    }
                }
            }
        }
    }
})
KOTLIN
export KRIT_GRADLE_PLUGIN_JAR=$plugin_jar

gradle_args=(kritExportModel --continue --no-watch-fs --init-script "$init_script")
if (cd "$corpus" && ./gradlew --help 2>/dev/null) | grep -q -- '--offline-if-possible'; then
  gradle_args+=(--offline-if-possible)
fi
echo "Exporting Gradle model for $corpus..." >&2
(cd "$corpus" && ./gradlew "${gradle_args[@]}") || fail "corpus Gradle model export failed: $corpus"

# Output in .krit is allowed; tracked changes anywhere in the corpus are not.
git -C "$corpus" rev-parse --show-toplevel >/dev/null 2>&1 || fail "cannot verify tracked-file status; corpus is not in a Git worktree: $corpus"
tracked_status=$(git -C "$corpus" status --porcelain --untracked-files=no -- .)
[[ -z $tracked_status ]] || fail "corpus has tracked-file changes after model export: $corpus
$tracked_status"
model_dir=$corpus/.krit/gradle-model
[[ -d $model_dir ]] || fail "model export produced no directory: $model_dir"
python3 - "$model_dir" <<'PY'
import glob, json, os, sys
files = sorted(glob.glob(os.path.join(sys.argv[1], '*.json')))
if not files:
    raise SystemExit('error: model export wrote no JSON files')
entries = 0
for path in files:
    with open(path, encoding='utf-8') as handle:
        model = json.load(handle)
    entries += sum(len(source.get('classpath', [])) for project in model.get('projects', []) for source in project.get('sourceSets', []))
print(f'MODEL_FILES={len(files)} CLASSPATH_ENTRIES={entries}')
PY

# slack-lints parity cases

`TestSlackLintParity` (`internal/rules/thirdparty_slack_lint_parity_test.go`) runs krit's ports of
[slack-lints](https://github.com/slackhq/slack-lints) 0.11.1 detectors over each case directory here and compares
the findings with `lint-0.11.1.txt`, the output of the real detector on the same sources. `divergences.txt` lists
deliberate differences, each with a reason. The MoshiUsage ports have their own fixtures under
`tests/fixtures/android-lint/MoshiUsage*/`.

A case directory holds `checks.txt` (the Lint IDs it checks), sources under `src/main/java/`, and the expected
files. `stubs/` holds minimal library sources (Dagger, javax.inject, RxAndroid's `AndroidSchedulers`) that Lint needs
to resolve the sources; krit does not read them. A case that needs real libraries on Lint's classpath lists them in
`jars.txt` as `group:artifact:version`.

## Regenerating `lint-0.11.1.txt`

The output comes from the standalone Lint CLI in directory mode (no Gradle project). Older Lint releases, such as
the 8.9.0 in `commandlinetools-linux-13114758`, silently skip the slack-lints registry.

```bash
# JDK 21
curl -fsSL -o jdk.tgz https://api.adoptium.net/v3/binary/latest/21/ga/linux/x64/jdk/hotspot/normal/eclipse
mkdir jdk && tar -xzf jdk.tgz -C jdk --strip-components=1
# Lint 9.4.0-rc01
curl -fsSL -o clt.zip https://dl.google.com/android/repository/commandlinetools-linux-16111833_latest.zip
python3 -I -c "import zipfile; zipfile.ZipFile('clt.zip').extractall('.')" && chmod +x cmdline-tools/bin/*
# slack-lint-checks and kotlin-stdlib
curl -fsSLO https://repo1.maven.org/maven2/com/slack/lint/slack-lint-checks/0.11.1/slack-lint-checks-0.11.1.jar
curl -fsSLO https://repo1.maven.org/maven2/org/jetbrains/kotlin/kotlin-stdlib/2.2.21/kotlin-stdlib-2.2.21.jar

export JAVA_HOME=$PWD/jdk LINT=$PWD/cmdline-tools/bin/lint \
  SLACK_LINT_JAR=$PWD/slack-lint-checks-0.11.1.jar KOTLIN_STDLIB_JAR=$PWD/kotlin-stdlib-2.2.21.jar
cd internal/rules/testdata/slack_lint
./run-lint.sh dagger stubs/dagger > dagger/lint-0.11.1.txt

# Cases with a jars.txt: download each listed artifact from Maven Central into one directory first.
export LINT_JARS_DIR=$PWD/jars && mkdir -p "$LINT_JARS_DIR"
while IFS=: read -r g a v; do
  curl -fsSL -o "$LINT_JARS_DIR/$a-$v.jar" "https://repo1.maven.org/maven2/${g//.//}/$a/$v/$a-$v.jar"
done < rx/jars.txt
./run-lint.sh rx stubs/rxandroid > rx/lint-0.11.1.txt
```

# slack-lints parity cases that need the resolver

`TestSlackLintResolverParity` (`internal/rules/thirdparty_view_checks_test.go`) runs like `TestSlackLintParity`
(see `../slack_lint/README.md`) but first indexes each case's sources with the source resolver, the way the pipeline
does, because these ports walk source class hierarchies. A case's `jars.txt` lists the Maven artifacts Lint needs
on its classpath (`libs/`); `stubs/` holds the library classes no published jar provides (RecyclerView's
`ItemDecoration`, `ViewBinding`, a generated binding). krit reads neither.

Regenerate a case's `lint-0.11.1.txt` with `../slack_lint/run-lint.sh <case> stubs/<case>`, with the `jars.txt`
artifacts on the Lint classpath.

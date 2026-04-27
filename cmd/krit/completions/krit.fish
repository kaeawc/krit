complete -c krit -l help -d 'Show help'
complete -c krit -l version -d 'Show version'
complete -c krit -s f -d 'Output format' -xa 'json sarif plain checkstyle'
complete -c krit -l report -d 'Output format (alias for -f)' -xa 'json sarif plain checkstyle'
complete -c krit -l config -d 'Config file' -rF
complete -c krit -l fix -d 'Apply auto-fixes'
complete -c krit -l fix-level -d 'Fix safety level' -xa 'cosmetic idiomatic semantic'
complete -c krit -l fix-suffix -d 'Write fixed files with suffix' -x
complete -c krit -l fix-binary -d 'Apply binary fixes'
complete -c krit -l dry-run -d 'Preview fixes'
complete -c krit -l remove-dead-code -d 'Bulk-remove directly fixable dead code findings'
complete -c krit -l all-rules -d 'Enable all rules'
complete -c krit -l no-cache -d 'Disable cache'
complete -c krit -l clear-cache -d 'Delete cache and exit'
complete -c krit -l cache-dir -d 'Shared cache directory' -xa '(__fish_complete_directories)'
complete -c krit -l no-type-inference -d 'Disable type inference'
complete -c krit -l no-type-oracle -d 'Disable type oracle'
complete -c krit -l daemon -d 'Enable persistent type oracle daemon'
complete -c krit -l oracle-diagnostics -d 'Collect Kotlin compiler diagnostics in the type oracle'
complete -c krit -l input-types -d 'Load pre-built type oracle JSON' -rF
complete -c krit -l output-types -d 'Output type oracle JSON' -rF
complete -c krit -l baseline -d 'Baseline file' -rF
complete -c krit -l create-baseline -d 'Create baseline file' -rF
complete -c krit -l base-path -d 'Base path for relative paths' -xa '(__fish_complete_directories)'
complete -c krit -l enable-editorconfig -d 'Read .editorconfig settings'
complete -c krit -l validate-config -d 'Validate config and exit'
complete -c krit -l list-rules -d 'List all rules'
complete -c krit -l generate-schema -d 'Print JSON Schema'
complete -c krit -l perf -d 'Performance timing'
complete -c krit -s v -d 'Verbose output'
complete -c krit -s q -d 'Suppress output'
complete -c krit -s j -d 'Number of parallel workers' -x
complete -c krit -s o -d 'Output file' -rF
complete -c krit -l warnings-as-errors -d 'Treat warnings as errors'
complete -c krit -l init -d 'Generate starter config'
complete -c krit -l doctor -d 'Check environment'
complete -c krit -l diff -d 'Only check changed files' -x
complete -c krit -l completions -d 'Print shell completions' -xa 'bash zsh fish'
complete -c krit -l disable-rules -d 'Comma-separated rules to disable' -x
complete -c krit -l enable-rules -d 'Comma-separated rules to enable' -x

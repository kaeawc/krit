#compdef krit

_krit() {
    _arguments \
        '--help[Show help]' \
        '--version[Show version]' \
        '-f[Output format]:format:(json sarif plain checkstyle)' \
        '--report[Output format (alias for -f)]:format:(json sarif plain checkstyle)' \
        '--config[Config file]:file:_files -g "*.yml"' \
        '--fix[Apply auto-fixes]' \
        '--fix-level[Fix safety level]:level:(cosmetic idiomatic semantic)' \
        '--fix-suffix[Write fixed files with suffix]:suffix:' \
        '--fix-binary[Apply binary fixes]' \
        '--dry-run[Preview fixes without applying]' \
        '--all-rules[Enable all rules]' \
        '--no-cache[Disable incremental cache]' \
        '--clear-cache[Delete cache and exit]' \
        '--cache-dir[Shared cache directory]:dir:_directories' \
        '--no-type-inference[Disable type inference]' \
        '--no-type-oracle[Disable type oracle]' \
        '--daemon[Enable persistent type oracle daemon]' \
        '--input-types[Load pre-built type oracle JSON]:file:_files -g "*.json"' \
        '--output-types[Output type oracle JSON]:file:_files -g "*.json"' \
        '--baseline[Baseline file]:file:_files -g "*.xml"' \
        '--create-baseline[Create baseline]:file:_files -g "*.xml"' \
        '--base-path[Base path for relative paths]:dir:_directories' \
        '--enable-editorconfig[Read .editorconfig settings]' \
        '--validate-config[Validate config file and exit]' \
        '--list-rules[List all rules]' \
        '--generate-schema[Print JSON Schema]' \
        '--perf[Show performance timing]' \
        '-v[Verbose output]' \
        '-q[Suppress non-finding output]' \
        '-j[Number of parallel workers]:jobs:' \
        '-o[Output file]:file:_files' \
        '--warnings-as-errors[Treat warnings as errors]' \
        '--init[Generate starter krit.yml]' \
        '--doctor[Check environment]' \
        '--diff[Only check files changed since git ref]:ref:' \
        '--completions[Print shell completions]:shell:(bash zsh fish)' \
        '--disable-rules[Comma-separated rules to disable]:rules:' \
        '--enable-rules[Comma-separated rules to enable]:rules:' \
        '*:path:_files -/'
}

_krit "$@"

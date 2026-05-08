_krit_completions() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    local prev="${COMP_WORDS[COMP_CWORD-1]}"

    case "$prev" in
        -f|--report)
            COMPREPLY=($(compgen -W "json sarif plain checkstyle" -- "$cur"))
            return
            ;;
        --config)
            COMPREPLY=($(compgen -f -X '!*.yml' -- "$cur"))
            return
            ;;
        --fix-level)
            COMPREPLY=($(compgen -W "cosmetic idiomatic semantic" -- "$cur"))
            return
            ;;
        --completions)
            COMPREPLY=($(compgen -W "bash zsh fish" -- "$cur"))
            return
            ;;
        --diff)
            COMPREPLY=($(compgen -W "HEAD~1 HEAD~5 main origin/main" -- "$cur"))
            return
            ;;
        --baseline|--create-baseline)
            COMPREPLY=($(compgen -f -X '!*.xml' -- "$cur"))
            return
            ;;
        --input-types|--output-types)
            COMPREPLY=($(compgen -f -X '!*.json' -- "$cur"))
            return
            ;;
        --cache-dir|--base-path)
            COMPREPLY=($(compgen -d -- "$cur"))
            return
            ;;
        -o)
            COMPREPLY=($(compgen -f -- "$cur"))
            return
            ;;
        -j)
            return
            ;;
    esac

    if [[ "$cur" == -* ]]; then
        COMPREPLY=($(compgen -W "--help --version -f --report --config --fix --fix-level --fix-suffix --fix-binary --dry-run --all-rules --no-cache --clear-cache --cache-dir --no-type-inference --no-type-oracle --daemon --input-types --output-types --baseline --create-baseline --base-path --enable-editorconfig --validate-config --list-rules --generate-schema --perf -v -q -j -o --warnings-as-errors --init --doctor --diff --completions --disable-rules --enable-rules" -- "$cur"))
        return
    fi

    # Default to directory/file completion
    COMPREPLY=($(compgen -d -- "$cur"))
}
complete -F _krit_completions krit

// Package prepush tests scripts/pre-push.sh, the local mirror of CI that
// .githooks/pre-push runs: its check planner against changed-file fixtures,
// and its merge-with-origin/main check against throwaway git repositories.
package prepush

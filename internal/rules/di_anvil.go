package rules

import (
	"fmt"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// AnvilMergeComponentEmptyScopeRule detects @MergeComponent scopes that have no
// matching @ContributesTo/@ContributesBinding declarations anywhere in the project.
type AnvilMergeComponentEmptyScopeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule. Detection uses annotation and import patterns for
// Dagger/Hilt/Anvil; project-specific DI aliases are not followed.
// Classified per roadmap/17.
func (r *AnvilMergeComponentEmptyScopeRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *AnvilMergeComponentEmptyScopeRule) check(ctx *api.Context) {
	index := ctx.CodeIndex
	if index == nil {
		return
	}
	files := index.Files
	if len(files) == 0 {
		return
	}

	contributedScopes := make(map[string]struct{})
	type mergeComponentCandidate struct {
		file  *scanner.File
		idx   uint32
		scope string
	}
	var mergeComponents []mergeComponentCandidate

	for _, file := range files {
		if file == nil || file.FlatTree == nil || !anvilMergeComponentMayMatch(file.Content) {
			continue
		}
		for _, idx := range anvilScopeDeclarationCandidates(file) {
			if !anvilModifiersMayMatchFlat(file, idx) {
				continue
			}

			if scope := anvilAnnotationScopeFlat(file, idx, "ContributesTo"); scope != "" {
				contributedScopes[scope] = struct{}{}
			}
			if scope := anvilAnnotationScopeFlat(file, idx, "ContributesBinding"); scope != "" {
				contributedScopes[scope] = struct{}{}
			}
			if scope := anvilAnnotationScopeFlat(file, idx, "MergeComponent"); scope != "" {
				mergeComponents = append(mergeComponents, mergeComponentCandidate{
					file:  file,
					idx:   idx,
					scope: scope,
				})
			}
		}
	}

	for _, candidate := range mergeComponents {
		if _, ok := contributedScopes[candidate.scope]; ok {
			continue
		}

		name := extractIdentifierFlat(candidate.file, candidate.idx)
		if name == "" {
			name = "merged component"
		}

		ctx.Emit(r.Finding(
			candidate.file,
			candidate.file.FlatRow(candidate.idx)+1,
			1,
			fmt.Sprintf("@MergeComponent(%s::class) on '%s' has no matching @ContributesTo or @ContributesBinding scope in the project, so the merged component will be empty.", candidate.scope, name),
		))
	}
}

// AnvilContributesBindingWithoutScopeRule detects a same-file mismatch between
// @ContributesBinding(scope) and the @ContributesTo(scope) on the bound interface.
type AnvilContributesBindingWithoutScopeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule. Detection uses annotation and import patterns for
// Dagger/Hilt/Anvil; project-specific DI aliases are not followed.
// Classified per roadmap/17.
func (r *AnvilContributesBindingWithoutScopeRule) Confidence() float64 { return api.ConfidenceMedium }

package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// GradleWrapperValidationActionRule flags Gradle workflow steps missing wrapper validation.
type GradleWrapperValidationActionRule struct {
	BaseRule
}

func (r *GradleWrapperValidationActionRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *GradleWrapperValidationActionRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{}
}

func (r *GradleWrapperValidationActionRule) check(ctx *api.Context) {
	if ctx.ModuleIndex == nil || ctx.ModuleIndex.Graph == nil {
		return
	}
	for _, path := range githubWorkflowFiles(ctx.ModuleIndex.Graph.RootDir) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, finding := range gradleWrapperValidationFindings(data) {
			ctx.Emit(scanner.Finding{
				File:       path,
				Line:       finding.line,
				Col:        1,
				RuleSet:    r.RuleSetName,
				Rule:       r.RuleName,
				Severity:   r.Sev,
				Message:    fmt.Sprintf("Gradle workflow step %s runs without a preceding wrapper validation step in the same job.", finding.uses),
				Confidence: r.Confidence(),
			})
		}
	}
}

type gradleWorkflowFinding struct {
	uses string
	line int
}

func githubWorkflowFiles(root string) []string {
	matches, _ := filepath.Glob(filepath.Join(root, ".github", "workflows", "*"))
	var out []string
	for _, path := range matches {
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yml" || ext == ".yaml" {
			out = append(out, path)
		}
	}
	sort.Strings(out)
	return out
}

func gradleWrapperValidationFindings(data []byte) []gradleWorkflowFinding {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || len(doc.Content) == 0 {
		return nil
	}
	jobs := yamlMappingValue(doc.Content[0], "jobs")
	if jobs == nil || jobs.Kind != yaml.MappingNode {
		return nil
	}

	var findings []gradleWorkflowFinding
	for i := 0; i+1 < len(jobs.Content); i += 2 {
		job := jobs.Content[i+1]
		steps := yamlMappingValue(job, "steps")
		if steps == nil || steps.Kind != yaml.SequenceNode {
			continue
		}
		seenValidation := false
		for _, step := range steps.Content {
			usesNode := yamlMappingValue(step, "uses")
			if usesNode == nil {
				continue
			}
			uses := strings.ToLower(strings.TrimSpace(usesNode.Value))
			if isGradleWrapperValidationAction(uses) {
				seenValidation = true
				continue
			}
			if isGradleBuildAction(uses) && !seenValidation {
				findings = append(findings, gradleWorkflowFinding{uses: usesNode.Value, line: usesNode.Line})
			}
		}
	}
	return findings
}

func yamlMappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func isGradleBuildAction(uses string) bool {
	return strings.HasPrefix(uses, "gradle/gradle-build-action@") ||
		strings.HasPrefix(uses, "gradle/actions/setup-gradle@")
}

func isGradleWrapperValidationAction(uses string) bool {
	return strings.HasPrefix(uses, "gradle/wrapper-validation-action@") ||
		strings.HasPrefix(uses, "gradle/actions/wrapper-validation@")
}

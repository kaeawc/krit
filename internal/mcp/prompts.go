package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// promptDefinitions returns the list of MCP prompt definitions.
func promptDefinitions() []PromptDefinition {
	return []PromptDefinition{
		{
			Name:        "review_kotlin",
			Description: "Review Kotlin code for issues and suggest improvements. Runs analysis and fix suggestions, then formats results as a code review.",
			Arguments: []PromptArgument{
				{
					Name:        "code",
					Description: "Kotlin source code to review",
					Required:    true,
				},
				{
					Name:        "path",
					Description: "File path (for context like package detection)",
					Required:    false,
				},
			},
		},
		{
			Name:        "prepare_pr",
			Description: "Analyze project changes for PR readiness. Runs full project analysis and summarizes findings suitable for a PR description.",
			Arguments: []PromptArgument{
				{
					Name:        "paths",
					Description: "Comma-separated list of directories or files to analyze",
					Required:    true,
				},
			},
		},
		{
			Name:        "refactor_check",
			Description: "Check if a refactoring is safe by finding symbol references and inspecting type dependencies.",
			Arguments: []PromptArgument{
				{
					Name:        "symbol",
					Description: "Symbol name to check references for",
					Required:    true,
				},
				{
					Name:        "code",
					Description: "Kotlin source code containing the symbol",
					Required:    false,
				},
				{
					Name:        "project_paths",
					Description: "Comma-separated list of directories to search for references",
					Required:    false,
				},
			},
		},
	}
}

// getPrompt resolves a prompt by name with the given arguments and returns the result.
func (s *Server) getPrompt(name string, arguments map[string]string) (*PromptGetResult, *RPCError) {
	switch name {
	case "review_kotlin":
		return s.promptReviewKotlin(arguments)
	case "prepare_pr":
		return s.promptPreparePR(arguments)
	case "refactor_check":
		return s.promptRefactorCheck(arguments)
	default:
		return nil, &RPCError{
			Code:    -32602,
			Message: "unknown prompt: " + name,
		}
	}
}

// promptReviewKotlin runs analyze + suggest_fixes and formats as a code review.
func (s *Server) promptReviewKotlin(arguments map[string]string) (*PromptGetResult, *RPCError) {
	code := arguments["code"]
	if code == "" {
		return nil, &RPCError{Code: -32602, Message: "'code' argument is required"}
	}
	path := arguments["path"]

	// Run analysis
	analyzeArgs, _ := json.Marshal(analyzeArgs{Code: code, Path: path})
	analyzeResult := s.toolAnalyze(analyzeArgs)

	// Run fix suggestions
	fixArgs, _ := json.Marshal(suggestFixesArgs{Code: code, Path: path, FixLevel: "all"})
	fixResult := s.toolSuggestFixes(fixArgs)

	// Build the review prompt
	var sb strings.Builder
	sb.WriteString("You are reviewing the following Kotlin code. Analyze the findings and fix suggestions below, then provide a thorough code review.\n\n")
	sb.WriteString("## Code Under Review\n```kotlin\n")
	sb.WriteString(code)
	sb.WriteString("\n```\n\n")
	sb.WriteString("## Static Analysis Findings\n```json\n")
	if len(analyzeResult.Content) > 0 {
		sb.WriteString(analyzeResult.Content[0].Text)
	}
	sb.WriteString("\n```\n\n")
	sb.WriteString("## Available Auto-Fixes\n```json\n")
	if len(fixResult.Content) > 0 {
		sb.WriteString(fixResult.Content[0].Text)
	}
	sb.WriteString("\n```\n\n")
	sb.WriteString("Based on these findings, provide:\n")
	sb.WriteString("1. A summary of the most important issues\n")
	sb.WriteString("2. Specific suggestions for improvement, referencing line numbers\n")
	sb.WriteString("3. Which auto-fixes are safe to apply\n")
	sb.WriteString("4. Any additional concerns not caught by the static analysis\n")

	return &PromptGetResult{
		Description: "Code review for Kotlin source",
		Messages: []PromptMessage{
			{
				Role:    "user",
				Content: ContentBlock{Type: "text", Text: sb.String()},
			},
		},
	}, nil
}

// promptPreparePR runs analyze_project and formats findings for a PR description.
func (s *Server) promptPreparePR(arguments map[string]string) (*PromptGetResult, *RPCError) {
	pathsStr := arguments["paths"]
	if pathsStr == "" {
		return nil, &RPCError{Code: -32602, Message: "'paths' argument is required"}
	}

	paths := strings.Split(pathsStr, ",")
	for i := range paths {
		paths[i] = strings.TrimSpace(paths[i])
	}

	// Run project analysis
	projectArgs, _ := json.Marshal(analyzeProjectArgs{Paths: paths, Format: "summary"})
	projectResult := s.toolAnalyzeProject(projectArgs)

	var sb strings.Builder
	sb.WriteString("You are preparing a pull request. Analyze the project analysis results below and help create a PR summary.\n\n")
	sb.WriteString("## Project Analysis Results\n```json\n")
	if len(projectResult.Content) > 0 {
		sb.WriteString(projectResult.Content[0].Text)
	}
	sb.WriteString("\n```\n\n")
	sb.WriteString("Based on these results, provide:\n")
	sb.WriteString("1. A summary of the current code quality state\n")
	sb.WriteString("2. Key issues that should be addressed before merging\n")
	sb.WriteString("3. A suggested PR description section about code quality\n")
	sb.WriteString("4. Any blocking issues vs. non-blocking suggestions\n")

	return &PromptGetResult{
		Description: "PR readiness analysis",
		Messages: []PromptMessage{
			{
				Role:    "user",
				Content: ContentBlock{Type: "text", Text: sb.String()},
			},
		},
	}, nil
}

// promptRefactorCheck runs find_references and optionally inspect_types.
func (s *Server) promptRefactorCheck(arguments map[string]string) (*PromptGetResult, *RPCError) {
	symbol := arguments["symbol"]
	if symbol == "" {
		return nil, &RPCError{Code: -32602, Message: "'symbol' argument is required"}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("You are checking whether it is safe to refactor the symbol '%s'. ", symbol))
	sb.WriteString("Review the reference search results and type information below.\n\n")

	// Run find_references if project_paths provided
	projectPathsStr := arguments["project_paths"]
	if projectPathsStr != "" {
		paths := strings.Split(projectPathsStr, ",")
		for i := range paths {
			paths[i] = strings.TrimSpace(paths[i])
		}
		refArgs, _ := json.Marshal(findReferencesArgs{
			Name:         symbol,
			ProjectPaths: paths,
			IncludeJava:  true,
			IncludeXML:   true,
		})
		refResult := s.toolFindReferences(refArgs)

		sb.WriteString("## Symbol References\n```json\n")
		if len(refResult.Content) > 0 {
			sb.WriteString(refResult.Content[0].Text)
		}
		sb.WriteString("\n```\n\n")
	}

	// Run inspect_types if code provided
	code := arguments["code"]
	if code != "" {
		typesArgs, _ := json.Marshal(inspectTypesArgs{Code: code, Query: "classes"})
		typesResult := s.toolInspectTypes(typesArgs)

		sb.WriteString("## Type Information\n```json\n")
		if len(typesResult.Content) > 0 {
			sb.WriteString(typesResult.Content[0].Text)
		}
		sb.WriteString("\n```\n\n")

		hierarchyArgs, _ := json.Marshal(inspectTypesArgs{Code: code, Query: "hierarchy"})
		hierarchyResult := s.toolInspectTypes(hierarchyArgs)

		sb.WriteString("## Type Hierarchy\n```json\n")
		if len(hierarchyResult.Content) > 0 {
			sb.WriteString(hierarchyResult.Content[0].Text)
		}
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString("Based on these results, provide:\n")
	sb.WriteString("1. All locations where the symbol is referenced\n")
	sb.WriteString("2. Whether the refactoring could break any callers\n")
	sb.WriteString("3. Type hierarchy dependencies that could be affected\n")
	sb.WriteString("4. A safety assessment: safe, risky, or dangerous\n")

	return &PromptGetResult{
		Description: fmt.Sprintf("Refactoring safety check for '%s'", symbol),
		Messages: []PromptMessage{
			{
				Role:    "user",
				Content: ContentBlock{Type: "text", Text: sb.String()},
			},
		},
	}, nil
}

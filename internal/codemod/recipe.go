package codemod

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Recipe struct {
	Name        string `yaml:"name"`
	Language    string `yaml:"language"`
	Match       string `yaml:"match"`
	Replacement string `yaml:"replacement"`
}

func LoadRecipe(path string) (Recipe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Recipe{}, err
	}
	var recipe Recipe
	if err := yaml.Unmarshal(data, &recipe); err != nil {
		return Recipe{}, err
	}
	if err := recipe.Validate(); err != nil {
		return Recipe{}, err
	}
	return recipe, nil
}

func (r Recipe) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("recipe name is required")
	}
	switch strings.TrimSpace(strings.ToLower(r.Language)) {
	case "kotlin", "java":
	default:
		return fmt.Errorf("language must be kotlin or java")
	}
	if strings.TrimSpace(r.Match) == "" {
		return fmt.Errorf("match query is required")
	}
	if r.Replacement == "" {
		return fmt.Errorf("replacement is required")
	}
	return nil
}

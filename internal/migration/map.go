package migration

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Map struct {
	Library    string      `yaml:"library" json:"library"`
	Migrations []Migration `yaml:"migrations" json:"migrations"`
}

type Migration struct {
	From    string  `yaml:"from" json:"from"`
	To      string  `yaml:"to" json:"to"`
	Symbols []Entry `yaml:"symbols" json:"symbols"`
}

type Entry struct {
	Symbol      string `yaml:"symbol" json:"symbol"`
	Replacement string `yaml:"replacement" json:"replacement"`
	Reason      string `yaml:"reason" json:"reason"`
}

func LoadMap(path string) (Map, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Map{}, err
	}
	var migrationMap Map
	if err := yaml.Unmarshal(data, &migrationMap); err != nil {
		return Map{}, fmt.Errorf("parse migration map: %w", err)
	}
	if err := migrationMap.Validate(); err != nil {
		return Map{}, err
	}
	return migrationMap, nil
}

func (m Map) Validate() error {
	if strings.TrimSpace(m.Library) == "" {
		return fmt.Errorf("migration map library is required")
	}
	if len(m.Migrations) == 0 {
		return fmt.Errorf("migration map requires at least one migration")
	}
	for _, migration := range m.Migrations {
		if strings.TrimSpace(migration.From) == "" || strings.TrimSpace(migration.To) == "" {
			return fmt.Errorf("migration from and to versions are required")
		}
		if len(migration.Symbols) == 0 {
			return fmt.Errorf("migration %s -> %s requires at least one symbol", migration.From, migration.To)
		}
		for _, entry := range migration.Symbols {
			if strings.TrimSpace(entry.Symbol) == "" {
				return fmt.Errorf("migration symbol is required")
			}
			if strings.TrimSpace(entry.Replacement) == "" {
				return fmt.Errorf("migration replacement is required for %s", entry.Symbol)
			}
		}
	}
	return nil
}

func (m Map) Select(library, from, to string) ([]Entry, error) {
	if library != "" && !strings.EqualFold(m.Library, library) {
		return nil, fmt.Errorf("migration map is for library %q, not %q", m.Library, library)
	}
	for _, migration := range m.Migrations {
		if migration.From == from && migration.To == to {
			return migration.Symbols, nil
		}
	}
	return nil, fmt.Errorf("no migration entries for %s %s -> %s", m.Library, from, to)
}

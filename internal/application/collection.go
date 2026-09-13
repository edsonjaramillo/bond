package application

import (
	"fmt"
	"path/filepath"
	"runtime"
)

type centralCollection string

const (
	skillCollection       centralCollection = "skills"
	resourceCollection    centralCollection = "resources"
	instructionCollection centralCollection = "instructions"
)

func centralCollectionPath(environment []string, collection centralCollection) (string, error) {
	if directory, ok := environmentValue(environment, "XDG_CONFIG_HOME"); ok && directory != "" {
		return filepath.Join(directory, "bond", string(collection)), nil
	}

	home, ok := environmentValue(environment, "HOME")
	if !ok || home == "" {
		return "", fmt.Errorf("resolve %s: HOME is not set", collection.displayName())
	}

	configurationDirectory := filepath.Join(home, ".config")
	if runtime.GOOS == "darwin" {
		configurationDirectory = filepath.Join(home, "Library", "Application Support")
	}

	return filepath.Join(configurationDirectory, "bond", string(collection)), nil
}

func (collection centralCollection) displayName() string {
	switch collection {
	case skillCollection:
		return "Store"
	case resourceCollection:
		return "Resource Store"
	case instructionCollection:
		return "Instruction Store"
	default:
		return "central collection"
	}
}

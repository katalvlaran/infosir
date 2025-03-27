package utils

import (
	"sync"

	"infosir/cmd/config"
)

// We can store a reference to the global config in a thread-safe manner if we want,
// but typically config.Cfg is accessible directly. We'll illustrate a convenience accessor:

var cfgOnce sync.Once

// GetConfig returns a pointer to the loaded config.Cfg (the global config).
// Some code might prefer calling config.Cfg directly, but this is an example.
func GetConfig() *config.Config {
	cfgOnce.Do(func() {
		// Optionally any one-time post-processing of config can happen here.
	})
	return &config.Cfg
}

// Additional Helper Functions could live here:

// e.g. FlattenStringArray: merges multiple arrays of strings
// func FlattenStringArray(arrays ...[]string) []string { ... }

// For referencing config.Cfg.Crypto.Pairs, we can do:
// pairs := GetConfig().Crypto.Pairs
//
// Some code might do other small utilities. Keep them here or in separate files as needed.

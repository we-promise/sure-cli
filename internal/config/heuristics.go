package config

import "github.com/spf13/viper"

// HeuristicsConfig holds all configurable heuristic parameters.
type HeuristicsConfig struct {
	Leaks LeaksConfig `json:"leaks"`
	Rules RulesConfig `json:"rules"`
}

type LeaksConfig struct {
	MinCount int     `json:"min_count"`
	MinTotal float64 `json:"min_total"`
	MaxAvg   float64 `json:"max_avg"`
}

type RulesConfig struct {
	MinConsistency float64 `json:"min_consistency"`
	MinOccurrences int     `json:"min_occurrences"`
}

// GetHeuristics returns the current heuristics configuration.
func GetHeuristics() HeuristicsConfig {
	return HeuristicsConfig{
		Leaks: LeaksConfig{
			MinCount: viper.GetInt("heuristics.leaks.min_count"),
			MinTotal: viper.GetFloat64("heuristics.leaks.min_total"),
			MaxAvg:   viper.GetFloat64("heuristics.leaks.max_avg"),
		},
		Rules: RulesConfig{
			MinConsistency: viper.GetFloat64("heuristics.rules.min_consistency"),
			MinOccurrences: viper.GetInt("heuristics.rules.min_occurrences"),
		},
	}
}

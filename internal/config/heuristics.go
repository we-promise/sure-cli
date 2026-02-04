package config

import "github.com/spf13/viper"

// HeuristicsConfig holds all configurable heuristic parameters.
type HeuristicsConfig struct {
	Fees          FeesConfig          `json:"fees"`
	Subscriptions SubscriptionsConfig `json:"subscriptions"`
	Leaks         LeaksConfig         `json:"leaks"`
	Rules         RulesConfig         `json:"rules"`
}

type FeesConfig struct {
	Keywords []string `json:"keywords"`
}

type SubscriptionsConfig struct {
	PeriodMinDays     float64 `json:"period_min_days"`
	PeriodMaxDays     float64 `json:"period_max_days"`
	WeeklyMinDays     float64 `json:"weekly_min_days"`
	WeeklyMaxDays     float64 `json:"weekly_max_days"`
	StddevMaxDays     float64 `json:"stddev_max_days"`
	AmountStddevRatio float64 `json:"amount_stddev_ratio"`
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
		Fees: FeesConfig{
			Keywords: viper.GetStringSlice("heuristics.fees.keywords"),
		},
		Subscriptions: SubscriptionsConfig{
			PeriodMinDays:     viper.GetFloat64("heuristics.subscriptions.period_min_days"),
			PeriodMaxDays:     viper.GetFloat64("heuristics.subscriptions.period_max_days"),
			WeeklyMinDays:     viper.GetFloat64("heuristics.subscriptions.weekly_min_days"),
			WeeklyMaxDays:     viper.GetFloat64("heuristics.subscriptions.weekly_max_days"),
			StddevMaxDays:     viper.GetFloat64("heuristics.subscriptions.stddev_max_days"),
			AmountStddevRatio: viper.GetFloat64("heuristics.subscriptions.amount_stddev_ratio"),
		},
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

// GetFeeKeywords returns configured fee keywords or empty slice (caller should use defaults).
func GetFeeKeywords() []string {
	return viper.GetStringSlice("heuristics.fees.keywords")
}

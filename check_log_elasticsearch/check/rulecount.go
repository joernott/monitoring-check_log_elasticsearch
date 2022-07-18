package check

import (
	"fmt"

	"github.com/joernott/nagiosplugin/v2"
	"github.com/rs/zerolog/log"
)

// This counts the hits per rule which are considers a match
type RuleCount map[string]RuleCountEntry

// Every Renty consists of a number of Hits and a slice of Contents from all the
// hits, which are output to Nagios/Icinga2
type RuleCountEntry struct {
	Count uint64   // Number of Hits
	Lines []string // Excerpt of data
}

// Extract just the number from the RuleCount map.
func (r RuleCount) Count(Name string) uint64 {
	return r[Name].Count
}

// Add a hit to the RuleCount entry with the given name. This increases the
// count and adds a maximum number of lines to the entry
func (r RuleCount) Add(Name string, Lines []string, MaxLines int) RuleCountEntry {
	logger := log.With().Str("func", "rulecount.Add").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")
	rule := r[Name]
	rule.Count++
	if Lines != nil {
		for _, line := range Lines {
			if len(rule.Lines) > MaxLines {
				break
			}
			rule.Lines = append(rule.Lines, line)
		}
	}
	return rule
}

// Outputs the RuleCountEntry to Nagios/Icinga2 as indented lines
func (r RuleCountEntry) OutputRuleCountLines(nagios *nagiosplugin.Check, MaxLines int) []string {
	logger := log.With().Str("func", "RuleCountEntry.OutputRuleCountLines").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")
	l := len(r.Lines)
	if l > MaxLines {
		l = MaxLines
	}
	for i := 0; i < l; i++ {
		nagios.AddLongPluginOutput(fmt.Sprintf("   %v", r.Lines[i]))
	}
	return r.Lines[0:l]
}

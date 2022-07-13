package check

import (
	"fmt"

	"github.com/joernott/nagiosplugin/v2"
	"github.com/rs/zerolog/log"
)

type RuleCountEntry struct {
	Count uint64
	Lines []string
}

type RuleCount map[string]RuleCountEntry

func (r RuleCount) Count(Name string) uint64 {
	return r[Name].Count
}

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

func (r RuleCountEntry) OutputRuleCountLines(nagios *nagiosplugin.Check, MaxLines int) {
	logger := log.With().Str("func", "RuleCountEntry.OutputRuleCountLines").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")
	l := len(r.Lines)
	if l > MaxLines {
		l = MaxLines
	}
	for i := 0; i < l; i++ {
		nagios.AddLongPluginOutput(fmt.Sprintf("   %v", r.Lines[i]))
	}
}

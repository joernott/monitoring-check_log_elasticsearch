package check

import (
	"regexp"

	//"github.com/davecgh/go-spew/spew"
	"github.com/joernott/monitoring-check_log_elasticsearch/check_log_elasticsearch/elasticsearch"
	"github.com/joernott/nagiosplugin/v2"
	"github.com/rs/zerolog/log"
)

// List of rules
type RuleList map[string]Rule

// Definition of a rule to apply on every hit from the Elastcsearch Search
// result.
type Rule struct {
	Description  string    `json:"description" yaml:"description"`     // Only used for documentation/readability purpose.
	MetricName   string    `json:"metric_name" yaml:"metric_name"`     // The rule name will be used as metric name unless overwritten here
	Order        int        `json:"order" yaml:"order"`                // Order for sorting the rules
	Pattern      []Pattern `json:"pattern" yaml:"pattern"`             // A list of patterns which are checked against the fields in the hit
	Exclude      []Pattern `json:"exclude" yaml:"exclude"`             // If a hit matches, the Exclude pattern are checked. If one of them matches, the hit will be considered not a match
	UseAnd       bool      `json:"use_and" yaml:"use_and"`             // If true, all Pattern must match (AND), otherwise one of the Pattern suffices (OR).
	StopOnMatch  bool      `json:"stop_on_match" yaml:"stop_on_match"` // Stop evaluating other rules if thhis rule matches
	Warning      string    `json:"warning" yaml:"warning"`             // Valid Nagios/Icinga range for the number of hits since the last time, the check was run
	Critical     string    `json:"critical" yaml:"critical"`           // Valid Nagios/Icinga range for the number of hits since the last time, the check was run
	OutputFields []string  `json:"output_fields" yaml:"output_fields"` // Which field content should be output to Nagios/Icinga
	OutputLines  int       `json:"output_lines" yaml:"output_lines"`   // Limits the number of lines to output
	warnRange    *nagiosplugin.Range
	critRange    *nagiosplugin.Range
}

// Pattern definition for Rules
type Pattern struct {
	Field string `json:"field" yaml:"field"` // Name of a Field in the hit from the Elasticsearch Search
	Regex string `json:"regex" yaml:"regex"` // GO regular expression to match
}

// Checks the provided Hit against the rule.
func (r Rule) isMatch(Hit elasticsearch.HitElement, DocumentId string, RuleName string) (bool, error) {
	logger := log.With().Str("func", "isMatch").Str("package", "check").Str("document_id", DocumentId).Str("rule", RuleName).Logger()
	logger.Trace().Msg("Enter func")
	found := false
	except := false
	first := true
	for _, p := range r.Pattern {
		s, ok := Hit.GetString(p.Field)
		if !ok {
			break
		}
		logger := logger.With().Str("field", p.Field).Str("value", s).Str("regex", p.Regex).Logger()
		match, err := regexp.MatchString(p.Regex, s)
		logger.Trace().Str("id", "DBG20040001").Bool("match", match).Msg("Checking pattern")
		if err != nil {
			logger.Error().Str("id", "ERR20040001").
				Str("field", s).
				Str("regex", p.Regex).
				Err(err).
				Msg("Could not match regex")
			return false, err
		}
		logger = logger.With().Bool("match", match).Bool("found", found).Logger()
		if r.UseAnd {
			if first {
				found = match
				first = false
				logger.Trace().Str("id", "DBG20040002").Bool("first", true).Bool("use_and", r.UseAnd).Msg("Result first and")
			} else {
				found = found && match
				logger.Trace().Str("id", "DBG20040003").Bool("first", false).Bool("use_and", r.UseAnd).Msg("Result not first and")
			}
		} else {
			found = match
			logger.Trace().Str("id", "DBG20040004").Bool("first", first).Bool("use_and", r.UseAnd).Msg("Result or")
			break
		}
	}
	if !found {
		logger.Trace().Str("id", "DBG20040005").Bool("found", found).Msg("Skip exception check")
		return found, nil
	}
	first = true
	for _, e := range r.Exclude {
		s, ok := Hit.GetString(e.Field)
		if !ok {
			break
		}
		b := []byte(s)
		logger = logger.With().Str("field", e.Field).Str("value", s).Str("regex", e.Regex).Logger()
		match, err := regexp.Match(e.Regex, b)
		logger.Trace().Str("id", "DBG20040006").Bool("except", match).Msg("Checking exclude")
		if err != nil {
			logger.Error().Str("id", "ERR20040002").Err(err).Msg("Could not match regex")
			return false, err
		}
		except = match
		if except {
			logger.Trace().Str("id", "DBG20040007").Bool("except", match).Msg("Hit exception for rule")
			break
		}
	}
	if except {
		return false, nil
	}
	return found, nil
}

// Generates a slice of field contents from an elasticsearch hit for the fields
// listed in rule.OutputFields
func (rule Rule) getOutputLines(hit elasticsearch.ElasticsearchHitList) []string {
	var lines []string
	if len(rule.OutputFields) > 0 {
		for _, field := range rule.OutputFields {
			data, ok := hit.Fields.GetString(field)
			if ok {
				lines = append(lines, data)
			}
		}
	}
	return lines
}

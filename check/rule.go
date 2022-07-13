package check

import (
	"regexp"

	//"github.com/davecgh/go-spew/spew"
	"github.com/joernott/monitoring-check_log_elasticsearch/elasticsearch"
	"github.com/joernott/nagiosplugin/v2"
	"github.com/rs/zerolog/log"
)

type Rule struct {
	Description  string    `json:"description" yaml:"description"`
	MetricName   string    `json:"metric_name" yaml:"metric_name"`
	Pattern      []Pattern `json:"pattern" yaml:"pattern"`
	Exclude      []Pattern `json:"exclude" yaml:"exclude"`
	UseAnd       bool      `json:"use_and" yaml:"use_and"`
	Warning      string    `json:"warning" yaml:"warning"`
	Critical     string    `json:"critical" yaml:"critical"`
	OutputFields []string  `json:"output_fields" yaml:"output_fields"`
	OutputLines  int       `json:"output_lines" yaml:"output_lines"`
	warnRange    *nagiosplugin.Range
	critRange    *nagiosplugin.Range
}

type Pattern struct {
	Field string `json:"field" yaml:"field"`
	Regex string `json:"regex" yaml:"regex"`
}

func (r Rule) isMatch(Source elasticsearch.HitElement) (bool, error) {
	logger := log.With().Str("func", "isMatch").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")
	found := false
	except := false
	first := true
	for _, p := range r.Pattern {
		s, ok := Source.GetString(p.Field)
		if !ok {
			break
		}
		match, err := regexp.MatchString(p.Regex, s)
		logger.Trace().Str("id", "DBG20040001").Str("field", p.Field).Str("value", s).Str("regex", p.Regex).Bool("match", match).Msg("Checking pattern")
		if err != nil {
			logger.Error().Str("id", "ERR20040001").
				Str("field", s).
				Str("regex", p.Regex).
				Err(err).
				Msg("Could not match regex")
			return false, err
		}
		if r.UseAnd {
			if first {
				found = match
				first = false
				logger.Trace().Str("id", "DBG20040002").Str("field", p.Field).Str("value", s).Str("regex", p.Regex).Bool("match", match).Bool("found", found).Bool("first", true).Bool("use_and", r.UseAnd).Msg("Result first and")
			} else {
				found = found && match
				logger.Trace().Str("id", "DBG20040003").Str("field", p.Field).Str("value", s).Str("regex", p.Regex).Bool("match", match).Bool("found", found).Bool("first", false).Bool("use_and", r.UseAnd).Msg("Result not first and")
			}
		} else {
			found = match
			logger.Trace().Str("id", "DBG20040004").Str("field", p.Field).Str("value", s).Str("regex", p.Regex).Bool("match", match).Bool("found", found).Bool("first", first).Bool("use_and", r.UseAnd).Msg("Result or")
			break
		}
	}
	if !found {
		logger.Trace().Str("id", "DBG20040005").Bool("found", found).Msg("Skip exception check")
		return found, nil
	}
	first = true
	for _, e := range r.Exclude {
		s, ok := Source.GetString(e.Field)
		if !ok {
			break
		}
		b := []byte(s)
		match, err := regexp.Match(e.Regex, b)
		logger.Trace().Str("id", "DBG20040006").Str("field", e.Field).Str("value", s).Str("regex", e.Regex).Bool("except", match).Msg("Checking exclude")
		if err != nil {
			logger.Error().Str("id", "ERR20040002").
				Str("field", s).
				Str("regex", e.Regex).
				Err(err).
				Msg("Could not match regex")
			return false, err
		}
		except = match
		if except {
			logger.Trace().Str("id", "DBG20040007").Str("field", e.Field).Str("value", s).Str("regex", e.Regex).Bool("except", match).Msg("Hit exception for rule")
			break
		}
	}
	if except {
		return false, nil
	}
	return found, nil
}


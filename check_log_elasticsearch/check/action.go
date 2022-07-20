package check

import (
	"errors"
	"fmt"
	"strings"
	"time"

	//"github.com/davecgh/go-spew/spew"
	"github.com/joernott/monitoring-check_log_elasticsearch/check_log_elasticsearch/elasticsearch"
	"github.com/joernott/nagiosplugin/v2"
	"github.com/rs/zerolog/log"
)

// Actions is the contents of an action file. It is a hash with currently only one element, an array auf Action objects
type Actions struct {
	Actions []Action `json:"actions" yaml:"actions"` // A list of actions
}

// Action specifies one action to be execuded by the check. Currently, only Elasticsearch queries are supported
type Action struct {
	Name           string          `json:"name" yaml:"name"`             // Name of the action
	History        uint64          `json:"history" yaml:"history"`       // Number of seconds to remember alarms
	Index          string          `json:"index" yaml:"index"`           // Index name or pattern
	Query          string          `json:"query" yaml:"query"`           // Query to be execuded
	Rules          map[string]Rule `json:"rule" yaml:"rules"`            // A list of rules to match the query results against
	Limit          uint            `json:"limit" yaml:"limit"`           // Limit to this number of pages (a page is 1000 hits) per call to the check. This is important for not overloading the elöasticsearch cluster or running into timeouts
	StatusFile     string          `json:"statusfile" yaml:"statusfile"` // Where to save the timestamp and history from this run for the next one
	last_timestamp string
	results        RuleCount
	StatusData     *StatusData
}

// countResults iterates over the data returned by an Elasticsearch search and checks for every hit (document) on which rule it matches
func (s Action) countResults(result *elasticsearch.ElasticsearchResult) (string, error) {
	var err error
	logger := log.With().Str("func", "Action.countResults").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")
	last_timestamp := ""
	for _, hit := range result.Hits.Hits {
		s.results.Add("_total", nil, 0)
		matches := false
		for rulename, rule := range s.Rules {
			match, err := rule.isMatch(hit.Fields)
			if err != nil {
				return "", err
			}
			logger.Trace().Str("id", "DBG20030001").Str("rule", rulename).Bool("match", match).Msg("Apply Rule")
			if match {
				matches = true
				lines := rule.getOutputLines(hit)
				s.results[rulename] = s.results.Add(rulename, lines, rule.OutputLines)
				if rule.StopOnMatch {
					break
				}
			}
		}
		if !matches {
			s.results.Add("_nomatch", nil, 0)
		}
		last_timestamp, err = getTimestamp(hit, "@timestamp")
		if err != nil {
			return "", err
		}
	}
	return last_timestamp, nil
}

// getTimestamp retrieves a timestamp from the elasticsearch hit
func getTimestamp(hit elasticsearch.ElasticsearchHitList, fieldname string) (string, error) {
	logger := log.With().Str("func", "getTimestamp").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")
	if fieldname == "" {
		fieldname = "@timestamp"
	}
	ts, found := hit.Fields.GetString(fieldname)
	if found {
		ts := strings.Replace(strings.Replace(ts, "[", "", -1), "]", "", -1)
		return ts, nil
	}
	ts, found = hit.Source.GetString(fieldname)
	if !found {
		err := errors.New("Document is missing field " + fieldname)
		logger.Error().Str("id", "ERR20030001").
			Str("field", fieldname).
			Err(err).
			Msg("Unsuitable data")
		return "", err
	}
	return ts, nil
}

// Generate the Nagios output for the current action
func (a Action) outputResults(nagios *nagiosplugin.Check) {
	logger := log.With().Str("func", "Action.outputResults").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")
	ts := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	for rulename, rule := range a.Rules {
		c := a.results.Count(rulename)
		logger := logger.With().Str("search", a.Name).Str("rule", rulename).Uint64("value", c).Logger()
		if rule.critRange.CheckUint64(c) {
			logger.Debug().Str("id", "DBG20080001").Str("threshold", rule.Critical).Str("type", "critical").Msg("Critical threshold reached")
			nagios.AddResult(nagiosplugin.CRITICAL, fmt.Sprintf("%v/%v", a.Name, rulename))
			nagios.AddLongPluginOutput(fmt.Sprintf("Value %v for rule %v in search %v exceeds threshold %v", c, rulename, a.Name, rule.Critical))
			lines := a.results[rulename].OutputRuleCountLines(nagios, rule.OutputLines)
			if a.History> 0 {
				a.StatusData.AddHistoryEntry(ts, int(nagiosplugin.CRITICAL), rulename, c, lines)
			}
		} else {
			if rule.warnRange.CheckUint64(c) {
				logger.Debug().Str("id", "DBG20080002").Str("threshold", rule.Warning).Str("type", "warning").Msg("Warning threshold reached")
				nagios.AddResult(nagiosplugin.WARNING, fmt.Sprintf("%v/%v", a.Name, rulename))
				nagios.AddLongPluginOutput(fmt.Sprintf("Value %v for rule %v in search %v exceeds threshold %v", c, rulename, a.Name, rule.Warning))
				lines := a.results[rulename].OutputRuleCountLines(nagios, rule.OutputLines)
				if a.History> 0 {
					a.StatusData.AddHistoryEntry(ts, int(nagiosplugin.WARNING), rulename, c, lines)
				}
			} else {
				logger.Debug().Str("id", "DBG20080003").Str("type", "ok").Msg("No threshold reached")
				nagios.AddResult(nagiosplugin.OK, fmt.Sprintf("%v/%v", a.Name, rulename))
				nagios.AddLongPluginOutput(fmt.Sprintf("Value %v for rule %v in search %v is within thresholds %v,%v	", c, rulename, a.Name, rule.Warning, rule.Critical))
			}
		}
		metric_name := rule.MetricName
		if metric_name == "" {
			metric_name = rulename
		}
		v, _ := nagiosplugin.NewFloatPerfDatumValue(float64(c))
		nagios.AddPerfDatum(metric_name, "c", v, rule.warnRange, rule.critRange, nil, nil)
	}
	t, _ := nagiosplugin.NewFloatPerfDatumValue(float64(a.results.Count("_total")))
	nagios.AddPerfDatum(a.Name+"_lines", "c", t, nil, nil, nil, nil)
	n, _ := nagiosplugin.NewFloatPerfDatumValue(float64(a.results.Count("_nomatch")))
	nagios.AddPerfDatum(a.Name+"_not_matched", "c", n, nil, nil, nil, nil)
	a.HistoricResults(nagios)
	return
}

// Generate the Nagios output for historic data stored in the status file
func (a Action) HistoricResults(nagios *nagiosplugin.Check) {
	var n nagiosplugin.Status
	var hc int
	logger := log.With().Str("func", "HistoricResults").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")
	if a.StatusData == nil {
		return
	}
	for _, h := range a.StatusData.History {
		if !h.Handled && !h.current {
			switch h.State {
			case 1:
				n = nagiosplugin.WARNING
			case 2:
				n = nagiosplugin.CRITICAL
			case 3:
				n = nagiosplugin.UNKNOWN
			default:
				n = nagiosplugin.OK
			}
			nagios.AddResult(n, fmt.Sprintf("%v/%v (historic)", a.Name, h.Rule))
			nagios.AddLongPluginOutput(fmt.Sprintf("Reporting unhandled historic event %v for rule %v for action %v which occurred on %v", h.Uuid, h.Rule, a.Name, h.Timestamp))
			for _, l := range h.Lines {
				nagios.AddLongPluginOutput(fmt.Sprintf("   %s", l))
			}
			nagios.AddLongPluginOutput("\n")
			hc++
		}
	}
	hp, _ := nagiosplugin.NewFloatPerfDatumValue(float64(hc))
	nagios.AddPerfDatum(a.Name+"_historic", "c", hp, nil, nil, nil, nil)
}

// Initialize a new Rulecount object based on the rules of this action
func (a Action) newRulecount() RuleCount {
	rulecount := make(RuleCount)
	for rulename := range a.Rules {
		rulecount[rulename] = RuleCountEntry{
			Count: 0,
		}
	}
	rulecount["_total"] = RuleCountEntry{
		Count: 0,
	}
	rulecount["_nomatch"] = RuleCountEntry{
		Count: 0,
	}
	return rulecount
}

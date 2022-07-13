package check

import (
	"errors"
	"fmt"
	"strings"
	"time"

	//"github.com/davecgh/go-spew/spew"
	"github.com/joernott/monitoring-check_log_elasticsearch/elasticsearch"
	"github.com/joernott/nagiosplugin/v2"
	"github.com/rs/zerolog/log"
)

type Actions struct {
	Actions []Action `json:"actions" yaml:"actions"`
}

type Action struct {
	Name           string          `json:"name" yaml:"name"`
	History        uint64          `json:"history" yaml:"history"`
	Index          string          `json:"index" yaml:"index"`
	Query          string          `json:"query" yaml:"query"`
	Rules          map[string]Rule `json:"rule" yaml:"rules"`
	Limit          uint            `json:"limit" yaml:"limit"`
	StatusFile     string          `json:"statusfile" yaml:"statusfile"`
	last_timestamp string
	results        RuleCount
	StatusData     *StatusData
}

func (s Action) countResults(result *elasticsearch.ElasticsearchResult) (string, error) {
	logger := log.With().Str("func", "countResults").Str("package", "check").Logger()
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
				var lines []string
				if len(rule.OutputFields) > 0 {
					for _, field := range rule.OutputFields {
						data, ok := hit.Fields.GetString(field)
						if ok {
							lines = append(lines, data)
						}
					}
				}
				s.results[rulename] = s.results.Add(rulename, lines, rule.OutputLines)
			}
		}
		if !matches {
			s.results.Add("_nomatch", nil, 0)
		}
		lt, found := hit.Fields.GetString("@timestamp")
		if found {
			last_timestamp = strings.Replace(strings.Replace(lt, "[", "", -1), "]", "", -1)
		} else {
			last_timestamp, found = hit.Fields.GetString("@timestamp")
			if !found {
				err := errors.New("Document is missing field @timestamp")
				logger.Error().Str("id", "ERR20030001").
					Str("field", "@timestamp").
					Err(err).
					Msg("Unsuitable data")

				return "", err
			}
		}
	}
	return last_timestamp, nil
}

func (a Action) outputResults(nagios *nagiosplugin.Check) error {
	logger := log.With().Str("func", "outputResults").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")
	ts := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	for rulename, rule := range a.Rules {
		c := a.results.Count(rulename)
		if rule.critRange.CheckUint64(c) {
			logger.Debug().
				Str("id", "DBG20080001").
				Str("search", a.Name).
				Str("rule", rulename).
				Str("threshold", rule.Critical).
				Str("type", "critical").
				Uint64("value", c).
				Msg("Critical threshold reached")
			nagios.AddResult(nagiosplugin.CRITICAL, fmt.Sprintf("%v/%v", a.Name, rulename))
			nagios.AddLongPluginOutput(fmt.Sprintf("Value %v for rule %v in search %v exceeds threshold %v", c, rulename, a.Name, rule.Critical))
			a.results[rulename].OutputRuleCountLines(nagios, rule.OutputLines)
			a.StatusData.AddHistoryEntry(ts, int(nagiosplugin.CRITICAL), rulename, c)
		} else {
			if rule.warnRange.CheckUint64(c) {
				logger.Debug().
					Str("id", "DBG20080002").
					Str("search", a.Name).
					Str("rule", rulename).
					Str("threshold", rule.Warning).
					Str("type", "warning").
					Uint64("value", c).
					Msg("Warning threshold reached")
				nagios.AddResult(nagiosplugin.WARNING, fmt.Sprintf("%v/%v", a.Name, rulename))
				nagios.AddLongPluginOutput(fmt.Sprintf("Value %v for rule %v in search %v exceeds threshold %v", c, rulename, a.Name, rule.Warning))
				a.results[rulename].OutputRuleCountLines(nagios, rule.OutputLines)
				a.StatusData.AddHistoryEntry(ts, int(nagiosplugin.WARNING), rulename, c)
			} else {
				logger.Debug().
					Str("id", "DBG20080003").
					Str("search", a.Name).
					Str("rule", rulename).
					Str("type", "ok").
					Uint64("value", c).
					Msg("No threshold reached")
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
	return nil
}

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
			hc++
		}
	}
	hp, _ := nagiosplugin.NewFloatPerfDatumValue(float64(hc))
	nagios.AddPerfDatum(a.Name+"_historic", "c", hp, nil, nil, nil, nil)
}

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

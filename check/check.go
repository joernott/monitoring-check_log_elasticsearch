package check

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"regexp"
	"strings"
	"time"

	//"github.com/davecgh/go-spew/spew"
	"github.com/davecgh/go-spew/spew"
	"github.com/joernott/monitoring-check_log_elasticsearch/elasticsearch"
	"github.com/olorin/nagiosplugin"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type Check struct {
	actionsFile string
	statusFile  string
	connection  *elasticsearch.Elasticsearch
	nagios      *nagiosplugin.Check
	actions     *Actions
}

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
	last_timestamp string
	results        RuleCount
	StatusData     *StatusData
}

type Rule struct {
	Description string    `json:"description" yaml:"description"`
	Order       int       `json:"order" yaml:"order"`
	MetricName  string    `json:"metric_name" yaml:"metric_name"`
	Pattern     []Pattern `json:"pattern" yaml:"pattern"`
	Exclude     []Pattern `json:"exclude" yaml:"exclude"`
	UseAnd      bool      `json:"use_and" yaml:"use_and"`
	Warning     string    `json:"warning" yaml:"warning"`
	Critical    string    `json:"critical" yaml:"critical"`
	warnRange   *nagiosplugin.Range
	critRange   *nagiosplugin.Range
}

type Pattern struct {
	Field string `json:"field" yaml:"field"`
	Regex string `json:"regex" yaml:"regex"`
}

type RuleCount map[string]uint64

func NewCheck(ActionsFile string, StatusFile string, Connection *elasticsearch.Elasticsearch, Nagios *nagiosplugin.Check) (*Check, error) {
	var actions *Actions
	var c *Check
	logger := log.With().Str("func", "NewCheck").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")
	c = new(Check)
	c.actionsFile = ActionsFile
	c.statusFile = StatusFile
	c.connection = Connection
	c.nagios = Nagios

	actions, err := readActionFile(ActionsFile)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(actions.Actions); i++ {
		for rulename, rule := range actions.Actions[i].Rules {
			r := rule
			r.warnRange, err = nagiosplugin.ParseRange(rule.Warning)
			if err != nil {
				logger.Error().
					Str("id", "ERR20000001").
					Str("search", actions.Actions[i].Name).
					Str("rule", rulename).
					Str("threshold", rule.Warning).
					Str("type", "warning").
					Err(err).
					Msg("Error parsing range")
				c.nagios.AddResult(nagiosplugin.UNKNOWN, "Error parsing warning range "+rule.Warning+" for rule "+rulename+" in search "+actions.Actions[i].Name)
				return nil, err
			}
			r.critRange, err = nagiosplugin.ParseRange(rule.Critical)
			if err != nil {
				logger.Error().
					Str("id", "ERR20000002").
					Str("search", actions.Actions[i].Name).
					Str("rule", rulename).
					Str("threshold", rule.Critical).
					Str("type", "critical").
					Err(err).
					Msg("Error parsing range")
				c.nagios.AddResult(nagiosplugin.UNKNOWN, "Error parsing critical range "+rule.Critical+" for rule "+rulename+" in search "+actions.Actions[i].Name)
				return nil, err
			}
			actions.Actions[i].Rules[rulename] = r
		}
		actions.Actions[i].results = actions.Actions[i].newRulecount()
	}
	c.actions = actions

	return c, nil
}

func readActionFile(ActionFile string) (*Actions, error) {
	var actions *Actions
	logger := log.With().Str("func", "readActionFile").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")

	logger.Debug().Str("id", "DBG20010001").Str("filename", ActionFile).Msg("Read action file")
	f, err := ioutil.ReadFile(ActionFile)
	if err != nil {
		logger.Fatal().Str("id", "ERR20010001").Err(err).Str("filename", ActionFile).Msg("Failed to read action file")
		return nil, err
	}
	actions = new(Actions)
	err = yaml.Unmarshal(f, actions)
	if err != nil {
		log.Fatal().Str("id", "ERR20010002").Str("file", ActionFile).Err(err).Msg("Error unmarshalling yaml config file")
		return nil, err
	}
	return actions, nil
}

func (c *Check) Execute(Actions []string) error {
	logger := log.With().Str("func", "Execute").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")

	for ac, a := range c.actions.Actions {
		if !actionInList(a.Name, Actions) {
			logger.Debug().Str("id", "DBG20020001").
				Str("name", a.Name).
				Str("index", a.Index).
				Str("query", a.Query).
				Msg("Search not in requested actions, skipping")
			continue
		}
		statusFile := c.statusFile + "_" + a.Name + ".yaml"
		s, err := ReadStatus(statusFile)
		if err != nil {
			logger.Error().
				Str("id", "ERR20000003").
				Str("filename", statusFile).
				Err(err).
				Msg("Error reading statusfile")
			c.nagios.AddResult(nagiosplugin.UNKNOWN, "Error reading timestamp from "+statusFile)
			return err
		}
		c.actions.Actions[ac].StatusData = s
		timestamp := s.Timestamp

		logger.Debug().Str("id", "DBG20020001").
			Str("name", a.Name).
			Str("index", a.Index).
			Str("query", a.Query).
			Str("timestamp", timestamp).
			Msg("Run search")
		q := strings.ReplaceAll(a.Query, "_TIMESTAMP_", timestamp)
		pagination, err := c.connection.StartPaginatedSearch(a.Index, q)
		if err != nil {
			logger.Error().Str("id", "ERR20020001").
				Str("name", a.Name).
				Str("index", a.Index).
				Str("query", a.Query).
				Str("timestamp", timestamp).
				Str("parsed_query", q).
				Int("page", 0).
				Str("reason", pagination.Results[0].Error.Reason).
				Err(err).
				Msg("Could not run search '" + a.Name + "'")
			c.nagios.AddResult(nagiosplugin.UNKNOWN, fmt.Sprintf("%v. Could not initiate paginated search %v on index %v. Query is %v", err, a.Name, a.Index, a.Query))
			return err
		}
		timestamp, err = a.countResults(pagination.Results[0])
		if err != nil {
			return err
		}
		c.actions.Actions[ac].StatusData.Timestamp = timestamp
		hc := len(pagination.Results[0].Hits.Hits)
		if hc < int(pagination.Pagination.Size) {
			logger.Info().Str("id", "INF20020001").Int("page", 0).Int("hits", hc).Str("timestamp", timestamp).Msg("Only page")
			break
		}
		logger.Info().Str("id", "INF20020001").Int("page", 0).Int("hits", hc).Str("timestamp", timestamp).Msg("First page")
		defer pagination.Close()
		for page := 0; page < int(a.Limit); page++ {
			err = pagination.Next()
			if err != nil {
				logger.Error().Str("id", "ERR20020002").
					Str("name", a.Name).
					Str("index", a.Index).
					Str("query", a.Query).
					Str("timestamp", timestamp).
					Str("parsed_query", q).
					Int("page", page).
					Str("reason", pagination.Results[len(pagination.Results)-1].Error.Reason).
					Err(err).
					Msg("Could not run paginated '" + a.Name + "'")
				c.nagios.AddResult(nagiosplugin.UNKNOWN, fmt.Sprintf("%v. Could not run paginated search %v #%v on index %v. Query is %v", err, a.Name, page, a.Index, a.Query))
				return err
			}
			timestamp, err = a.countResults(pagination.Results[len(pagination.Results)-1])
			if err != nil {
				return err
			}
			c.actions.Actions[ac].StatusData.Timestamp = timestamp
			hc := len(pagination.Results[len(pagination.Results)-1].Hits.Hits)
			if hc < int(pagination.Pagination.Size) {
				logger.Info().Str("id", "INF20020001").Int("page", page).Int("hits", hc).Str("timestamp", timestamp).Msg("Last page")
				break
			}
			logger.Info().Str("id", "INF20020001").Int("page", page).Int("hits", hc).Str("timestamp", timestamp).Msg("Next page")
		}
	}
	c.outputAll()
	return nil
}

func (s Action) countResults(result *elasticsearch.ElasticsearchResult) (string, error) {
	logger := log.With().Str("func", "countResults").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")
	last_timestamp := ""
	for _, hit := range result.Hits.Hits {
		s.results["_total"]++
		matches := false
		for rulename, rule := range s.Rules {
			match, err := rule.isMatch(hit.Source)
			if err != nil {
				return "", err
			}
			logger.Trace().Str("id", "DBG20030001").Str("rule", rulename).Bool("match", match).Msg("Apply Rule")
			if match {
				matches = true
				s.results[rulename]++
			}
		}
		if !matches {
			s.results["_nomatch"]++
		}
		lt, found := hit.Fields.Get("@timestamp")
		if found {
			last_timestamp = strings.Replace(strings.Replace(lt, "[", "", -1), "]", "", -1)
		} else {
			last_timestamp, found = hit.Source.Get("@timestamp")
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

func (r Rule) isMatch(Source elasticsearch.HitElement) (bool, error) {
	logger := log.With().Str("func", "isMatch").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")
	found := false
	except := false
	first := true
	for _, p := range r.Pattern {
		s, ok := Source.Get(p.Field)
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
		s, ok := Source.Get(e.Field)
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

func actionInList(action string, list []string) bool {
	if len(list) == 0 {
		return true
	}
	for _, a := range list {
		if a == action {
			return true
		}
	}
	return false
}

func (a Action) outputResults(nagios *nagiosplugin.Check) error {
	logger := log.With().Str("func", "outputResults").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")
	ts := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	for rulename, rule := range a.Rules {
		if rule.critRange.CheckUint64(a.results[rulename]) {
			logger.Debug().
				Str("id", "DBG20080001").
				Str("search", a.Name).
				Str("rule", rulename).
				Str("threshold", rule.Critical).
				Str("type", "critical").
				Uint64("value", a.results[rulename]).
				Msg("Critical threshold reached")
			nagios.AddResult(nagiosplugin.CRITICAL, fmt.Sprintf("Value %v for rule %v in search %v exceeds threshold %v", a.results[rulename], rulename, a.Name, rule.Critical))
			a.StatusData.AddHistoryEntry(ts, int(nagiosplugin.CRITICAL), rulename, a.results[rulename])
		} else {
			if rule.warnRange.CheckUint64(a.results[rulename]) {
				logger.Debug().
					Str("id", "DBG20080002").
					Str("search", a.Name).
					Str("rule", rulename).
					Str("threshold", rule.Warning).
					Str("type", "warning").
					Uint64("value", a.results[rulename]).
					Msg("Warning threshold reached")
				nagios.AddResult(nagiosplugin.WARNING, fmt.Sprintf("Value %v for rule %v in search %v exceeds threshold %v", a.results[rulename], rulename, a.Name, rule.Warning))
				a.StatusData.AddHistoryEntry(ts, int(nagiosplugin.WARNING), rulename, a.results[rulename])
			} else {
				logger.Debug().
					Str("id", "DBG20080003").
					Str("search", a.Name).
					Str("rule", rulename).
					Str("type", "ok").
					Uint64("value", a.results[rulename]).
					Msg("No threshold reached")
				nagios.AddResult(nagiosplugin.OK, fmt.Sprintf("Value %v for rule %v in search %v is within thresholds %v,%v	", a.results[rulename], rulename, a.Name, rule.Warning, rule.Critical))
			}
		}
		metric_name := rule.MetricName
		if metric_name == "" {
			metric_name = rulename
		}
		nagios.AddPerfDatum(metric_name, "c", float64(a.results[rulename]), math.Inf(1), math.Inf(1), rule.warnRange.End, rule.critRange.End)
	}
	nagios.AddPerfDatum(a.Name+"_lines", "c", float64(a.results["_total"]), math.Inf(1), math.Inf(1), math.Inf(1), math.Inf(1))
	nagios.AddPerfDatum(a.Name+"_not_matched", "c", float64(a.results["_nomatch"]), math.Inf(1), math.Inf(1), math.Inf(1), math.Inf(1))
	return nil
}

func (c Check) outputAll() error {
	for _, a := range c.actions.Actions {
		if a.History > 0 {
			spew.Dump(a.StatusData)
			a.StatusData.Prune(a.History)
		}
		err := a.outputResults(c.nagios)
		if err != nil {
			return err
		}
		statusFile := c.statusFile + "_" + a.Name + ".yaml"
		err = a.StatusData.Save(statusFile)
		if err != nil {
			c.nagios.AddResult(nagiosplugin.UNKNOWN, fmt.Sprintf("Could not save last timestamp %v to %v, error %v", a.StatusData.Timestamp, statusFile, err))
			return err
		}
	}
	return nil
}

func (a Action) newRulecount() RuleCount {
	rulecount := make(RuleCount)
	for rulename := range a.Rules {
		rulecount[rulename] = 0
	}
	rulecount["_total"] = 0
	rulecount["_nomatch"] = 0
	return rulecount
}

func (c *Check) GetAction(Name string) (Action, error) {
	logger := log.With().Str("func", "GetAction").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")
	for _, a := range c.actions.Actions {
		if a.Name == Name {
			return a, nil
		}
	}
	err := errors.New("Action " + Name + "not found")
	logger.Error().Str("id", "ERR20110001").Str("name", Name).Err(err).Msg("Get action failed")
	return Action{}, err
}

func (c *Check) ClearHistory(Actions []string, Uuids []string) error {
	logger := log.With().Str("func", "ClearHistory").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")
	for _, a := range c.actions.Actions {
		if !actionInList(a.Name, Actions) {
			logger.Debug().Str("id", "DBG20120001").
				Str("name", a.Name).
				Str("index", a.Index).
				Str("query", a.Query).
				Msg("Search not in requested actions, skipping")
			continue
		}
		statusFile := c.statusFile + "_" + a.Name + ".yaml"
		s, err := ReadStatus(statusFile)
		if err != nil {
			log.Error().Str("id", "20120001").Str("filename", statusFile).Err(err).Msg("Could not read status file")
			return err
		}
		logger.Debug().Str("id", "DBG20120002").Str("filename", statusFile).Msg("Read status file")
		s.RemoveHistoryEntry(Uuids)
		err = s.Save(statusFile)
		if err != nil {
			return err
		}
	}
	return nil
}

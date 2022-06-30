package check

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"regexp"
	"strings"

	//"github.com/davecgh/go-spew/spew"
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
	Index          string          `json:"index" yaml:"index"`
	Query          string          `json:"query" yaml:"query"`
	Rules          map[string]Rule `json:"rule" yaml:"rules"`
	last_timestamp string
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
	}
	c.actions = actions

	return c, nil
}

func readActionFile(ActionFile string) (*Actions, error) {
	var actions *Actions
	logger := log.With().Str("func", "readActionFile").Str("package", "check").Logger()

	logger.Debug().Str("id", "DBG20010001").Str("file", ActionFile).Msg("Read action file")
	f, err := ioutil.ReadFile(ActionFile)
	if err != nil {
		logger.Fatal().Str("id", "ERR20010001").Err(err).Str("file", ActionFile).Msg("Failed to read action file")
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
	for _, s := range c.actions.Actions {
		if !actionInList(s.Name, Actions) {
			logger.Debug().Str("id", "DBG20020001").
				Str("name", s.Name).
				Str("index", s.Index).
				Str("query", s.Query).
				Msg("Search not in requested actions, skipping")
			continue
		}
		statusFile := c.statusFile + "_" + s.Name
		timestamp, err := readTimestamp(statusFile)
		if err != nil {
			logger.Error().
				Str("id", "ERR20000003").
				Str("filename", statusFile).
				Err(err).
				Msg("Error reading timestamp from statusfile")
			c.nagios.AddResult(nagiosplugin.UNKNOWN, "Error reading timestamp from "+statusFile)
			return err
		}

		logger.Debug().Str("id", "DBG20020001").
			Str("name", s.Name).
			Str("index", s.Index).
			Str("query", s.Query).
			Str("timestamp", timestamp).
			Msg("Run search")
		q := strings.ReplaceAll(s.Query, "_TIMESTAMP_", timestamp)
		result, err := c.connection.Search(s.Index, q)
		if err != nil {
			logger.Error().Str("id", "ERR20020001").
				Str("name", s.Name).
				Str("index", s.Index).
				Str("query", s.Query).
				Str("timestamp", timestamp).
				Str("parsed_query", q).
				Str("reason", result.Error.Reason).
				Err(err).
				Msg("Could not run search '" + s.Name + "'")
			c.nagios.AddResult(nagiosplugin.UNKNOWN, fmt.Sprintf("%v. Could not run search %v on index %v. Query is %v", err, s.Name, s.Index, s.Query))
			return err
		}
		timestamp, rulecount, err := s.countResults(result)
		if err != nil {
			return err
		}
		err = saveTimestamp(timestamp, statusFile)
		if err != nil {
			c.nagios.AddResult(nagiosplugin.UNKNOWN, fmt.Sprintf("Could not save last timestamp %v to %v, error %v", timestamp, statusFile, err))
			return err
		}
		s.outputResults(c.nagios, rulecount)
	}
	return nil
}

func (s Action) countResults(result *elasticsearch.ElasticsearchResult) (string, RuleCount, error) {
	var found bool
	logger := log.With().Str("func", "countResults").Str("package", "check").Logger()
	rulecount := make(RuleCount)
	rulecount["_total"] = 0
	rulecount["_nomatch"] = 0
	last_timestamp := ""
	for _, hit := range result.Hits.Hits {
		rulecount["_total"]++
		matches := false
		for rulename, rule := range s.Rules {
			logger.Debug().Str("id", "DBG20030001").Str("rule", rulename).Msg("Apply Rule")
			match, err := rule.isMatch(hit.Source)
			if err != nil {
				return "", nil, err
			}
			logger.Debug().Str("id", "DBG20030001").Str("rule", rulename).Bool("match", match).Msg("Apply Rule")
			if match {
				matches = true
				rulecount[rulename]++
			} else {
				rulecount[rulename] += 0
			}
		}
		if !matches {
			rulecount["_nomatch"]++
		}
		last_timestamp, found = hit.Source.Get("@timestamp")
		if !found {
			err := errors.New("Document is missing field @timestamp")
			logger.Error().Str("id", "ERR20030001").
				Str("field", "@timestamp").
				Err(err).
				Msg("Unsuitable data")

			return "", nil, err
		}
	}
	return last_timestamp, rulecount, nil
}

func (r Rule) isMatch(Source elasticsearch.HitElement) (bool, error) {
	logger := log.With().Str("func", "isMatch").Str("package", "check").Logger()
	found := false
	except := false
	first := true
	for _, p := range r.Pattern {
		s, ok := Source.Get(p.Field)
		if !ok {
			break
		}
		match, err := regexp.MatchString(p.Regex, s)
		logger.Debug().Str("id", "DBG20040001").Str("field", p.Field).Str("value", s).Str("regex", p.Regex).Bool("match", match).Msg("Checking pattern")
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
				logger.Debug().Str("id", "DBG20040002").Str("field", p.Field).Str("value", s).Str("regex", p.Regex).Bool("match", match).Bool("found", found).Bool("first", true).Bool("use_and", r.UseAnd).Msg("Result first and")
			} else {
				found = found && match
				logger.Debug().Str("id", "DBG20040003").Str("field", p.Field).Str("value", s).Str("regex", p.Regex).Bool("match", match).Bool("found", found).Bool("first", false).Bool("use_and", r.UseAnd).Msg("Result not first and")
			}
		} else {
			found = match
			logger.Debug().Str("id", "DBG20040004").Str("field", p.Field).Str("value", s).Str("regex", p.Regex).Bool("match", match).Bool("found", found).Bool("first", first).Bool("use_and", r.UseAnd).Msg("Result or")
			break
		}
	}
	if !found {
		logger.Debug().Str("id", "DBG20040005").Bool("found", found).Msg("Skip exception check")
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
		logger.Debug().Str("id", "DBG20040002").Str("field", e.Field).Str("value", s).Str("regex", e.Regex).Bool("except", match).Msg("Checking exclude")
		if err != nil {
			logger.Error().Str("id", "ERR20040002").
				Str("field", s).
				Str("regex", e.Regex).
				Err(err).
				Msg("Could not match regex")
			return false, err
		}
		if r.UseAnd {
			if first {
				except = match
				first = false
			} else {
				found = found && match
			}
		} else {
			except = match
			break
		}
	}
	if except {
		return false, nil
	}
	return found, nil
}

func saveTimestamp(ts string, filename string) error {
	logger := log.With().Str("func", "saveTimestamp").Str("package", "check").Logger()

	if ts == "" {
		logger.Debug().Str("id", "DBG2005001").Msg("Skipped writing empty timestamp to file")
		return nil
	}

	f, err := os.Create(filename)

	if err != nil {
		logger.Error().Str("id", "ERR2005001").Str("filename", filename).Err(err).Msg("Could not create file")
		return err
	}
	defer f.Close()

	_, err = f.WriteString(ts)
	if err != nil {
		logger.Error().Str("id", "ERR2005002").Str("filename", filename).Err(err).Msg("Could not create file")
		return err
	}
	logger.Debug().Str("id", "DBG2005002").Str("timestamp", ts).Str("filename", filename).Msg("Wrote timestamp to file")
	return nil
}

func readTimestamp(filename string) (string, error) {
	var timestamp string
	logger := log.With().Str("func", "readTimestamp").Str("package", "check").Logger()

	if _, err := os.Stat(filename); err == nil {
		ts, err := os.ReadFile(filename)
		if err != nil {
			logger.Error().Str("id", "ERR2006001").Str("filename", filename).Err(err).Msg("Could not read from file")
			return "", err
		}
		timestamp = string(ts[:])
		logger.Debug().Str("id", "DBG2006001").Str("timestamp", timestamp).Str("filename", filename).Msg("Read timestamp from file")
	} else if errors.Is(err, os.ErrNotExist) {
		timestamp = "1900-01-01T00:00:00.000Z"
		logger.Debug().Str("id", "DBG2006002").Str("timestamp", timestamp).Str("filename", filename).Msg("No state file found, using default start date")

	} else {
		logger.Error().Str("id", "ERR2006002").Str("filename", filename).Err(err).Msg("Could not stat file")
		// Schrodinger: file may or may not exist. See err for details.
		return "", err
	}
	return timestamp, nil
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

func (a Action) outputResults(nagios *nagiosplugin.Check, rulecount RuleCount) error {
	logger := log.With().Str("func", "outputResults").Str("package", "check").Logger()

	for rulename, rule := range a.Rules {
		if rule.critRange.CheckUint64(rulecount[rulename]) {
			logger.Debug().
				Str("id", "DBG20080001").
				Str("search", a.Name).
				Str("rule", rulename).
				Str("threshold", rule.Critical).
				Str("type", "critical").
				Uint64("value", rulecount[rulename]).
				Msg("Critical threshold reached")
			nagios.AddResult(nagiosplugin.CRITICAL, fmt.Sprintf("Value %v for rule %v in search %v exceeds threshold %v", rulecount[rulename], rulename, a.Name, rule.Critical))
		} else {
			if rule.warnRange.CheckUint64(rulecount[rulename]) {
				logger.Debug().
					Str("id", "DBG20080002").
					Str("search", a.Name).
					Str("rule", rulename).
					Str("threshold", rule.Warning).
					Str("type", "warning").
					Uint64("value", rulecount[rulename]).
					Msg("Warning threshold reached")
				nagios.AddResult(nagiosplugin.WARNING, fmt.Sprintf("Value %v for rule %v in search %v exceeds threshold %v", rulecount[rulename], rulename, a.Name, rule.Warning))
			} else {
				logger.Debug().
					Str("id", "DBG20080003").
					Str("search", a.Name).
					Str("rule", rulename).
					Str("type", "ok").
					Uint64("value", rulecount[rulename]).
					Msg("No threshold reached")
				nagios.AddResult(nagiosplugin.OK, fmt.Sprintf("Value %v for rule %v in search %v is within thresholds %v,%v	", rulecount[rulename], rulename, a.Name, rule.Warning, rule.Critical))
			}
		}
		metric_name := rule.MetricName
		if metric_name == "" {
			metric_name = rulename
		}
		nagios.AddPerfDatum(metric_name, "c", float64(rulecount[rulename]), math.Inf(1), math.Inf(1), rule.warnRange.End, rule.critRange.End)
	}
	nagios.AddPerfDatum("lines", "c", float64(rulecount["_total"]), math.Inf(1), math.Inf(1), math.Inf(1), math.Inf(1))
	nagios.AddPerfDatum("not_matched", "c", float64(rulecount["_nomatch"]), math.Inf(1), math.Inf(1), math.Inf(1), math.Inf(1))
	return nil
}

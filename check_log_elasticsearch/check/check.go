package check

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	//"github.com/davecgh/go-spew/spew"
	"github.com/joernott/monitoring-check_log_elasticsearch/check_log_elasticsearch/elasticsearch"
	"github.com/joernott/nagiosplugin/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

//The Check object created and initialized by NewCheck consolidates the
// connection to Elasticsearch, the nagios object and the actions loaded from
// the action file
type Check struct {
	actionsFile string
	connection  *elasticsearch.Elasticsearch
	nagios      *nagiosplugin.Check
	actions     *Actions
	Command     string
}

// Creates a Check object containing the connection object to Elasticsearch, a
// Nagios object This also initializes and loads the action data structure from
// the Actions file
func NewCheck(ActionsFile string, Connection *elasticsearch.Elasticsearch, Nagios *nagiosplugin.Check, Command string) (*Check, error) {
	var actions *Actions
	var c *Check
	var o OrderedRuleList
	
	logger := log.With().Str("func", "NewCheck").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")
	c = new(Check)
	c.actionsFile = ActionsFile
	c.connection = Connection
	c.nagios = Nagios
	c.Command = Command

	actions, err := readActionFile(ActionsFile)
	if err != nil {
		c.nagios.AddResult(nagiosplugin.UNKNOWN, "Error parsing action file "+ActionsFile+": "+err.Error())
		return nil, err
	}
	for i := 0; i < len(actions.Actions); i++ {
		for rulename, rule := range actions.Actions[i].Rules {
			r := rule
			logger := logger.With().Str("search", actions.Actions[i].Name).Str("rule", rulename).Logger()
			r.warnRange, err = nagiosplugin.ParseRange(rule.Warning)
			if err != nil {
				logger.Error().Str("id", "ERR20000001").
					Str("threshold", rule.Warning).
					Str("type", "warning").
					Err(err).
					Msg("Error parsing range")
				c.nagios.AddResult(nagiosplugin.UNKNOWN, "Error parsing warning range "+rule.Warning+" for rule "+rulename+" in search "+actions.Actions[i].Name)
				return nil, err
			}
			r.critRange, err = nagiosplugin.ParseRange(rule.Critical)
			if err != nil {
				logger.Error().Str("id", "ERR20000002").
					Str("threshold", rule.Critical).
					Str("type", "critical").
					Err(err).
					Msg("Error parsing range")
				c.nagios.AddResult(nagiosplugin.UNKNOWN, "Error parsing critical range "+rule.Critical+" for rule "+rulename+" in search "+actions.Actions[i].Name)
				return nil, err
			}
			actions.Actions[i].Rules[rulename] = r
			o = o.Append(rulename,r.Order)
		}
		actions.Actions[i].results = actions.Actions[i].newRulecount()
		actions.Actions[i].orderedRules = o.Sort()
	}
	c.actions = actions
	return c, nil
}

// Read the actions data from the provided actions file
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

// Execute all Actions listed in the Actions parameter. If it is empty, all
// actions are executed
func (c *Check) Execute(Actions []string) error {
	var found bool
	logger := log.With().Str("func", "Execute").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")

	for ac, a := range c.actions.Actions {
		logger := logger.With().Str("name", a.Name).Str("index", a.Index).Str("query", a.Query).Logger()
		if !actionInList(a.Name, Actions) {
			logger.Debug().Str("id", "DBG20020001").
				Msg("Search not in requested actions, skipping")
			continue
		}
		found = true
		s, err := ReadStatus(a.StatusFile)
		if err != nil {
			logger.Error().Str("id", "ERR20000003").
				Str("filename", a.StatusFile).
				Err(err).
				Msg("Error reading statusfile")
			c.nagios.AddResult(nagiosplugin.UNKNOWN, "Error reading timestamp from "+a.StatusFile)
			return err
		}
		c.actions.Actions[ac].StatusData = s
		timestamp := s.Timestamp

		logger.Debug().Str("id", "DBG20020001").Str("timestamp", timestamp).Msg("Run search")
		q := strings.ReplaceAll(a.Query, "_TIMESTAMP_", timestamp)
		pagination, err := c.connection.StartPaginatedSearch(a.Index, q)
		if err != nil {
			reason := ""
			if pagination != nil {
				if len(pagination.Results) > 0 {
					reason = pagination.Results[0].Error.Reason
				}
			}
			logger.Error().Str("id", "ERR20020001").
				Str("timestamp", timestamp).
				Str("parsed_query", q).
				Int("page", 0).
				Str("reason", reason).
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
		for page := 0; page < int(a.Limit-1); page++ {
			err = pagination.Next()
			if err != nil {
				logger.Error().Str("id", "ERR20020002").
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
	if !found {
		arr := zerolog.Arr()
		for _, a := range Actions {
			arr.Str(a)
		}
		logger.Error().Str("id", "ERR20020003").
			Str("error", "Actions don't match action file").
			Array("actions", arr).
			Str("file", c.actionsFile).
			Msg("None of the actions were found in the action file")
		c.nagios.AddResult(nagiosplugin.UNKNOWN, fmt.Sprintf("None of the actions %v were found in the action file %v", Actions, c.actionsFile))
	}
	c.outputAll(Actions)
	return nil
}

// Little helper looking if the action is in the given list. Also returns true,
// if the list is empty
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

// Output all results for all actions
func (c *Check) outputAll(Actions []string) error {
	logger := log.With().Str("func", "Check.outputAll").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")
	for i, a := range c.actions.Actions {
		if !actionInList(a.Name, Actions) {
			logger.Debug().Str("id", "DBG20120001").
				Str("name", a.Name).
				Str("index", a.Index).
				Str("query", a.Query).
				Msg("Search not in requested actions, skipping")
			continue
		}
		if a.History > 0 {
			c.actions.Actions[i].StatusData.Prune(a.History)
		}
		a.outputResults(c.nagios, c.Command)
		err := c.actions.Actions[i].StatusData.Save(a.StatusFile)
		if err != nil {
			c.nagios.AddResult(nagiosplugin.UNKNOWN, fmt.Sprintf("Could not save last timestamp %v to %v, error %v", a.StatusData.Timestamp, a.StatusFile, err))
			return err
		}
	}
	return nil
}

// Get a specific action by name
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

// This sets the "handled" flag for a list of historic events. The Actions
// parameter specifies the actions to look for, if it is empty, all actions will
// be checked. The parameter Uuids is a list of all the historic events to
// change.
func (c *Check) HandleHistory(Actions []string, Uuids []string, All bool) error {
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
		s, err := ReadStatus(a.StatusFile)
		if err != nil {
			log.Error().Str("id", "ERR20120001").Str("filename", a.StatusFile).Err(err).Msg("Could not read status file")
			return err
		}
		s.HandleHistoryEntry(Uuids, All)
		err = s.Save(a.StatusFile)
		if err != nil {
			return err
		}
	}
	return nil
}

// This removes entries from a list of historic events. The Actions
// parameter specifies the actions to look for, if it is empty, all actions will
// be checked. The parameter Uuids is a list of all the historic events to
// delete
func (c *Check) RmHistory(Actions []string, Uuids []string, All bool) error {
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
		s, err := ReadStatus(a.StatusFile)
		if err != nil {
			log.Error().Str("id", "ERR20120001").Str("filename", a.StatusFile).Err(err).Msg("Could not read status file")
			return err
		}
		s.RemoveHistoryEntry(Uuids, All)
		err = s.Save(a.StatusFile)
		if err != nil {
			return err
		}
	}
	return nil
}

// Lists historic entries from the status files of the given actions. If the
// Actions parameter is empty, all Actions are looked at.
func (c *Check) ListHistory(Actions []string, HighlightUuid bool) error {
	logger := log.With().Str("func", "ListHistory").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")
	for _, a := range c.actions.Actions {
		if !actionInList(a.Name, Actions) {
			logger.Debug().Str("id", "DBG20130001").
				Str("name", a.Name).
				Str("index", a.Index).
				Str("query", a.Query).
				Msg("Search not in requested actions, skipping")
			continue
		}
		s, err := ReadStatus(a.StatusFile)
		if err != nil {
			log.Error().Str("id", "ERR20130001").Str("filename", a.StatusFile).Err(err).Msg("Could not read status file")
			return err
		}
		fmt.Println(a.Name)
		s.PrintHistory("", true, "", HighlightUuid, c.Command)
	}
	return nil
}

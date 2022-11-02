package check

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// The information stored in the status file.
type StatusData struct {
	Timestamp string          `json:"timestamp" yaml:"timestamp"` // a Timestamp in the format expected by Elasticsearch in the timestamp field e.g. 1900-01-01T00:00:00.000Z
	History   []StatusHistory `json:"history" yaml:"history"`     // Slice of historic events.
}

// A StatusHistory entry has a Uuid, a Timestamp, when it happened, the
// resulting State for Nagios/Icinga2 and the name of the Rule.
// The bool field Handled can be set to true, if the Event has been handled
// and should not be used for alerting again. The Counter is the numebr of
// Hits for that rule.
// current will be used to skip over the "historic" events added during the
// current run.
type StatusHistory struct {
	Uuid      string   `json:"uuid" yaml:"uuid"`           // Generated when adding a history entry, used for management
	Timestamp string   `json:"timestamp" yaml:"timestamp"` //The timestamp of the check
	State     int      `json:"state" yaml:"state"`         // State reported to Icinga/Nagios
	Rule      string   `json:"rule" yaml:"rule"`           // Name of the rule which triggered the alarm
	Handled   bool     `json:"handled" yaml:"handled"`     // If set to true, mark this historic entry as handled
	Counter   uint64   `json:"counter" yaml:"counter"`     // Number of lines matching the rule
	Lines     []string `json:"lines" yaml:"lines"`         // An except of the matchinmg lines
	current   bool
}

// Saves the StatusData structure to the given file
func (data *StatusData) Save(Filename string) error {
	logger := log.With().Str("func", "StatusData.Save").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")

	if data.Timestamp == "" {
		logger.Debug().Str("id", "DBG2101001").Msg("Skipped writing empty timestamp to file")
		return nil
	}
	yaml, err := yaml.Marshal(data)
	if err != nil {
		logger.Error().Str("id", "ERR2101001").Str("filename", Filename).Err(err).Msg("Could marshal yaml")
		return err
	}

	f, err := os.Create(Filename)
	if err != nil {
		logger.Error().Str("id", "ERR2101002").Str("filename", Filename).Err(err).Msg("Could not create file")
		return err
	}
	defer f.Close()

	_, err = f.Write(yaml)
	if err != nil {
		logger.Error().Str("id", "ERR2101003").Str("filename", Filename).Err(err).Msg("Could not write to file")
		return err
	}
	logger.Debug().Str("id", "DBG2101002").Str("timestamp", data.Timestamp).Str("filename", Filename).Msg("Wrote status file")
	return nil
}

// Loads the StatusData from the given file. If the file does not exist, an
// empty structure with a timestamp of "1900-01-01T00:00:00.000Z" is returned.
// If the file can't be read, an error is returned.
func ReadStatus(Filename string) (*StatusData, error) {
	var data *StatusData

	logger := log.With().Str("func", "readStatus").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")

	data = new(StatusData)

	if _, err := os.Stat(Filename); err == nil {
		f, err := ioutil.ReadFile(Filename)
		if err != nil {
			logger.Error().Str("id", "ERR2102001").Str("filename", Filename).Err(err).Msg("Could not read from file")
			return nil, err
		}
		err = yaml.Unmarshal(f, data)
		if err != nil {
			log.Fatal().Str("id", "ERR20010002").Str("filename", Filename).Err(err).Msg("Error unmarshalling yaml config file")
			return nil, err
		}
		logger.Debug().Str("id", "DBG2102001").Str("timestamp", data.Timestamp).Str("filename", Filename).Msg("Read status file")
	} else if errors.Is(err, os.ErrNotExist) {
		data.Timestamp = "1900-01-01T00:00:00.000Z"
		logger.Debug().Str("id", "DBG1002002").Str("timestamp", data.Timestamp).Str("filename", Filename).Msg("No state file found, using default start date")
	} else {
		logger.Error().Str("id", "ERR2102002").Str("filename", Filename).Err(err).Msg("Could not stat status file")
		// Schrodinger: file may or may not exist. See err for details.
		return nil, err
	}
	return data, nil
}

// Add a history entry to the StatusData Object
func (status *StatusData) AddHistoryEntry(Timestamp string, State int, Rule string, Counter uint64, Lines []string) {
	h := StatusHistory{
		Uuid:      uuid.New().String(),
		Timestamp: Timestamp,
		State:     State,
		Handled:   false,
		Rule:      Rule,
		Counter:   Counter,
		Lines:     Lines,
		current:   true,
	}
	status.History = append(status.History, h)
}

// Sets the Handled field to true for the historic entry with the given Uuid.
func (status *StatusData) Acknowledge(Uuid string) {
	for _, h := range status.History {
		if Uuid == h.Uuid {
			h.Handled = true
		}
	}
}

// Iterates over all historic events and removes them, if they are older than
// the provided retention time in seconds
func (status *StatusData) Prune(Retention uint64) {
	logger := log.With().Str("func", "status.Prune").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")

	format := "2006-01-02T15:04:05.000Z"
	var new []StatusHistory
	for _, h := range status.History {
		ts, err := time.Parse(format, h.Timestamp)
		if err != nil {
			logger.Warn().Str("id", "WRN1005001").Str("timestamp", h.Timestamp).Str("format", format).Str("uuid", h.Uuid).Msg("Could not interpret timestamp while pruning history")
			continue
		}
		s := time.Since(ts).Seconds()
		if s < float64(Retention) {
			logger.Trace().Float64("seconds_since_event", s).Uint64("retention", Retention).Str("uuid", h.Uuid).Msg("Retention not reached")
			new = append(new, h)
		} else {
			logger.Trace().Float64("seconds_since_event", s).Uint64("retention", Retention).Str("uuid", h.Uuid).Msg("Retention reached, removing " + h.Uuid)
		}
	}
	status.History = new
}

// Removes one or more Entries contained in the list of Uuids from the history
// of the StatusData.
func (status *StatusData) RemoveHistoryEntry(Uuids []string, All bool) {
	var new []StatusHistory

	logger := log.With().Str("func", "status.RemoveHistoryEntry").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")

	for _, h := range status.History {
		found := All
		if found {
			logger.Trace().Str("id", "DBG1006001").Str("uuid", h.Uuid).Msg("Removing all uuids")
		} else {
			for _, u := range Uuids {
				if h.Uuid == u {
					logger.Trace().Str("id", "DBG1006001").Str("uuid", u).Msg("Removing uuid")
					found = true
					break
				} else {
					logger.Trace().Str("id", "DBG1006002").Str("uuid", u).Str("compare_to", h.Uuid).Msg("No match")
				}
			}
		}
		if !found {
			new = append(new, h)
		}
	}
	status.History = new
}

// Sets one or more Entries contained in the list of Uuids from the history
// of the StatusData to "handled".
func (status *StatusData) HandleHistoryEntry(Uuids []string, All bool) {
	var new []StatusHistory

	logger := log.With().Str("func", "status.RemoveHistoryEntry").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")

	for _, h := range status.History {
		found := All
		if found {
			logger.Trace().Str("id", "DBG1006001").Str("uuid", h.Uuid).Msg("Handle all uuids")
		} else {
			for _, u := range Uuids {
				if h.Uuid == u {
					logger.Trace().Str("id", "DBG1006001").Str("uuid", u).Msg("Handle uuid")
					found = true
					break
				} else {
					logger.Trace().Str("id", "DBG1006002").Str("uuid", u).Str("compare_to", h.Uuid).Msg("No match")
				}
			}
		}
		if !found {
			h.Handled = true
		}
		new = append(new, h)
	}
	status.History = new
}

// Print a formatted list of history entries to stdout.
func (status *StatusData) PrintHistory(Format string, Caption bool, CaptionFormat string, HighlightUuid bool) {
	logger := log.With().Str("func", "status.PrintHistory").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")

	if Format == "" {
		Format = "%-36s %-24s %-8s %6d %1s %-16s\n"
	}
	if CaptionFormat == "" {
		CaptionFormat = "%-36s %-24s %-8s %6s %1s %-16s\n"
	}

	states := [4]string{"OK", "WARNING", "CRITICAL", "UNKNOWN"}
	if Caption {
		fmt.Printf(CaptionFormat, "UUID", "Date/Time", "State", "#", "Handled", "Rule")
	}
	logger.Trace().Int("count", len(status.History)).Msg("Entries to list")

	for _, h := range status.History {
		handled := "N"
		if h.Handled {
			handled = "Y"
		}
		u:=h.Uuid
		if HighlightUuid {
			u="\033[1m" + u + "\033[0m"
		}
		fmt.Printf(Format, u, h.Timestamp, states[h.State], h.Counter, handled, h.Rule)
		for _, l := range h.Lines {
			fmt.Printf("   %s\n", l)
		}
	}
}

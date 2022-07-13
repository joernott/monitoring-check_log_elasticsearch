package check

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	//"github.com/davecgh/go-spew/spew"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type StatusData struct {
	Timestamp string          `json:"timestamp" yaml:"timestamp"`
	History   []StatusHistory `json:"history" yaml:"history"`
}

type StatusHistory struct {
	Uuid      string `json:"uuid" yaml:"uuid"`
	Timestamp string `json:"timestamp" yaml:"timestamp"`
	State     int    `json:"state" yaml:"state"`
	Rule      string `json:"rule" yaml:"rule"`
	Handled   bool   `json:"handled" yaml:"handled"`
	Counter   uint64 `json:"counter" yaml:"counter"`
	current   bool
}

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

func (status *StatusData) AddHistoryEntry(Timestamp string, State int, Rule string, Counter uint64) {
	h := StatusHistory{
		Uuid:      uuid.New().String(),
		Timestamp: Timestamp,
		State:     State,
		Handled:   false,
		Rule:      Rule,
		Counter:   Counter,
		current:   true,
	}
	status.History = append(status.History, h)
}

func (status *StatusData) Acknowledge(Uuid string) {
	for _, h := range status.History {
		if Uuid == h.Uuid {
			h.Handled = true
		}
	}
}

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
		if time.Since(ts).Seconds() < float64(Retention) {
			new = append(new, h)
		}
	}
	status.History = new
}

func (status *StatusData) RemoveHistoryEntry(Uuids []string) {
	var new []StatusHistory

	logger := log.With().Str("func", "status.RemoveHistoryEntry").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")

	for _, h := range status.History {
		found := false
		for _, u := range Uuids {
			if h.Uuid == u {
				logger.Trace().Str("id", "DBG1006001").Str("uuid", u).Msg("Removing uuid")
				found = true
				break
			} else {
				logger.Trace().Str("id", "DBG1006002").Str("uuid", u).Str("compare_to", h.Uuid).Msg("No match")
			}
		}
		if !found {
			new = append(new, h)
		}
	}
	status.History = new
}

func (status *StatusData) HandleHistoryEntry(Uuids []string) {
	var new []StatusHistory

	logger := log.With().Str("func", "status.RemoveHistoryEntry").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")

	for _, h := range status.History {
		found := false
		for _, u := range Uuids {
			if h.Uuid == u {
				logger.Trace().Str("id", "DBG1006001").Str("uuid", u).Msg("Removing uuid")
				found = true
				break
			} else {
				logger.Trace().Str("id", "DBG1006002").Str("uuid", u).Str("compare_to", h.Uuid).Msg("No match")
			}
		}
		if !found {
			h.Handled=true
		}
		new = append(new, h)
	}
	status.History = new
}

func (status *StatusData) PrintHistory(Format string, Caption bool, CaptionFormat string) {
	logger := log.With().Str("func", "status.PrintHistory").Str("package", "check").Logger()
	logger.Trace().Msg("Enter func")

	if Format == "" {
		Format = "%-36s %-24s %-8s %6d %1s %-16s\n"
	}
	if CaptionFormat == "" {
		CaptionFormat = "%-36s %-24s %-8s %-1s %6s %-16s\n"
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
		fmt.Printf(Format, h.Uuid, h.Timestamp, states[h.State], h.Counter, handled, h.Rule)
	}
}

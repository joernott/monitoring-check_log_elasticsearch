package elasticsearch

import (
	"encoding/json"

	//"github.com/davecgh/go-spew/spew"
	"github.com/rs/zerolog/log"
)

type ElasticsearchPitResponse struct {
	Id    string             `json:"id"`
	Error ElasticsearchError `json:"error"`
}

type ElasticsearchPit struct {
	Id        string `json:"id"`
	KeepAlive string `json:"keep_alive"`
}

func (e *Elasticsearch) Pit(Index string, KeepAlive string) (string, error) {
	var x []byte
	logger := log.With().Str("func", "Pit").Str("package", "elasticsearch").Logger()
	ResultJson := new(ElasticsearchPitResponse)
	endpoint := "/" + Index + "/_pit?keep_alive=" + KeepAlive

	logger.Debug().Str("id", "DBG10040001").Str("index", Index).Str("keepalive", KeepAlive).Str("endpoint", endpoint).Msg("Get Point In Time")

	result, err := e.Connection.Post(endpoint, x)
	if result != nil {
		err2 := json.Unmarshal(result, ResultJson)
		if err2 != nil {
			logger.Error().Str("id", "ERR10040001").Str("data", string(result[:])).Err(err).Msg("Unmarshal failed")
			return "", err2
		}
	}
	if err != nil {
		logger.Error().Str("id", "ERR10040002").Err(err).Str("reason", ResultJson.Error.Reason).Msg("PIT failed")
		return "", err
	}
	pit := ResultJson.Id
	logger.Info().Str("id", "INF10040001").Str("index", Index).Str("keepalive", KeepAlive).Str("pit", pit).Str("endpoint", endpoint).Msg("Successfully got a pit")
	return pit, nil
}

func (e *Elasticsearch) DeletePit(Pit string) error {
	logger := log.With().Str("func", "DeletePit").Str("package", "elasticsearch").Logger()
	ResultJson := new(ElasticsearchErrorResponse)
	endpoint := "/_pit"
	s := "{\"id\":\"" + Pit + "\"}"

	logger.Debug().Str("id", "DBG10050001").Str("pit", Pit).Str("endpoint", endpoint).Msg("Delete Point In Time")
	result, err := e.Connection.Delete(endpoint, []byte(s))
	if result != nil {
		err2 := json.Unmarshal(result, ResultJson)
		if err2 != nil {
			logger.Error().Str("id", "ERR10050001").Err(err).Msg("Unmarshal failed")
			return err2
		}
	}
	if err != nil {
		logger.Error().Str("id", "ERR10050002").Err(err).Str("reason", ResultJson.Error.Reason).Msg("Delete PIT failed")
		return err
	}
	logger.Info().Str("id", "INF10050001").Str("pit", Pit).Str("endpoint", endpoint).Msg("Successfully deleted pit")
	return nil
}

// package elasticsearch handles the interaction with elasticsearch
package elasticsearch

import (
	"github.com/joernott/lra"
	"github.com/rs/zerolog/log"
)

type Elasticsearch struct {
	Connection *lra.Connection
	Endpoint   string
	Query      string
}

func NewElasticsearch(SSL bool, Host string, Port string, User string, Password string, ValidateSSL bool, Proxy string, Socks bool) (*Elasticsearch, error) {
	var e *Elasticsearch

	logger := log.With().Str("func", "NewElasticsearch").Str("package", "elasticsearch").Logger()
	e = new(Elasticsearch)

	hdr := make(lra.HeaderList)
	hdr["Content-Type"] = "application/json"
	c, err := lra.NewConnection(SSL,
		Host,
		Port,
		User,
		Password,
		"",
		ValidateSSL,
		Proxy,
		Socks,
		hdr)
	if err != nil {
		logger.Error().Err(err)
		return nil, err
	}
	e.Connection = c
	return e, nil
}

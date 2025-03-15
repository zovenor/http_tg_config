package http_tg_config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/invopop/jsonschema"
)

type validator interface {
	Validate() error
}

type updater[T any] interface {
	Update(T) error
}

type Config[T any] interface {
	validator
	updater[T]
}

type configHandler[T Config[T]] struct {
	logger *slog.Logger
	http.Handler
	cfg T
}

func NewConfigHandler[T Config[T]](cfg T, parentMux *http.ServeMux, logger *slog.Logger) *configHandler[T] {
	if logger == nil {
		logger = slog.Default()
	}
	s := &configHandler[T]{
		cfg:    cfg,
		logger: logger,
	}

	mux := http.NewServeMux()
	if parentMux != nil {
		mux = parentMux
	}

	mux.HandleFunc("/config/", s.serveConfig)
	mux.HandleFunc("/config-schema/", s.serveSchema)

	s.Handler = mux
	return s
}

func (s *configHandler[T]) serveSchema(w http.ResponseWriter, _ *http.Request) {
	schema := jsonschema.Reflect(s.cfg)
	bytes, err := schema.MarshalJSON()
	if err != nil {
		err := fmt.Errorf("failed to marshal schema: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		slog.Warn(err.Error())
		return
	}
	w.Write(bytes)
}

func (s *configHandler[T]) serveConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		bytes, err := json.Marshal(s.cfg)
		if err != nil {
			err = fmt.Errorf("failed to marshal config: %w", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(bytes)
	case http.MethodPost:
		decoder := json.NewDecoder(r.Body)
		var newCfg T
		if err := decoder.Decode(newCfg); err != nil {
			err = fmt.Errorf("failed to decode request body: %w", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := newCfg.Validate(); err != nil {
			err = fmt.Errorf("failed to validate request body: %w", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.cfg.Update(newCfg); err != nil {
			err = fmt.Errorf("failed to update config: %w", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	case http.MethodOptions:
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

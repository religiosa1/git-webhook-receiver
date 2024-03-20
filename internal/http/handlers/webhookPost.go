package handlers

import (
	"log"
	"log/slog"
	"net/http"

	"github.com/religiosa1/deployer/internal/config"
	"github.com/religiosa1/deployer/internal/wh_receiver"
)

func HandleWebhookPost(logger *slog.Logger, cfg *config.Config, receiver wh_receiver.Receiver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		action, err := receiver.ExtractAction(req)
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
		}
		// TODO
		log.Printf("%+v", action)
		w.WriteHeader(http.StatusOK)
	}
}

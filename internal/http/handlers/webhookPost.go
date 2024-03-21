package handlers

import (
	"errors"
	"io"
	"log"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/religiosa1/deployer/internal/config"
	"github.com/religiosa1/deployer/internal/wh_receiver"
)

func HandleWebhookPost(logger *slog.Logger, project *config.Project, receiver wh_receiver.Receiver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		webhookInfo, err := receiver.GetWebhookInfo(req)
		if err != nil {
			if _, ok := err.(wh_receiver.IncorrectRepoError); ok {
				logger.Error("Incorrect repo posted in the webhook", err)
				w.WriteHeader(http.StatusUnprocessableEntity)
				return
			}
			if errors.Is(err, io.EOF) {
				logger.Error("Empty body supplied in the webhook request")
				w.WriteHeader(http.StatusUnprocessableEntity)
				return
			}
			logger.Error("Error while parsing the webhook request payload", err)
		}
		w.WriteHeader(http.StatusOK)

		pipeLogger := logger.With(slog.String("pipeId", uuid.NewString())).With(slog.Any("webhookInfo", webhookInfo))
		go processWebHookPost(pipeLogger, project, webhookInfo)
	}
}

func processWebHookPost(logger *slog.Logger, project *config.Project, webhookInfo *wh_receiver.WebhookPostInfo) {
	logger.Debug("Recieved a webhook post")
	actions := filterOutAction(project, webhookInfo)
	if len(actions) == 0 {
		logger.Info("No applicable actions found in webhook post")
	}
	for _, action := range actions {
		log.Printf("%+v", action)
		// TODO
	}
}

func filterOutAction(project *config.Project, webhookInfo *wh_receiver.WebhookPostInfo) []config.Action {
	actions := make([]config.Action, 0)
	for _, action := range project.Actions {
		if action.Branch != webhookInfo.Branch {
			continue
		}
		if action.On != "*" && action.On != webhookInfo.Event {
			continue
		}
		actions = append(actions, action)
	}
	return actions
}

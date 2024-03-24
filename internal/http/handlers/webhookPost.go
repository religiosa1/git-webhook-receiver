package handlers

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/religiosa1/webhook-receiver/internal/config"
	"github.com/religiosa1/webhook-receiver/internal/whreceiver"
)

func HandleWebhookPost(
	logger *slog.Logger,
	cfg *config.Config,
	project *config.Project,
	receiver whreceiver.Receiver,
) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		webhookInfo, err := receiver.GetWebhookInfo(req)
		if err != nil {
			if _, ok := err.(whreceiver.IncorrectRepoError); ok {
				logger.Error("Incorrect repo posted in the webhook", slog.Any("error", err))
				w.WriteHeader(http.StatusUnprocessableEntity)
				return
			}
			if errors.Is(err, io.EOF) {
				logger.Error("Empty body supplied in the webhook request")
				w.WriteHeader(http.StatusUnprocessableEntity)
				return
			}
			logger.Error("Error while parsing the webhook request payload", slog.Any("error", err))
		}
		w.WriteHeader(http.StatusOK)

		hookLogger := logger.With(slog.Any("webhookInfo", webhookInfo))
		go processWebHookPost(hookLogger, cfg.ActionsOutputDir, project, webhookInfo)
	}
}

func processWebHookPost(
	logger *slog.Logger,
	actions_output_dir string,
	project *config.Project,
	webhookInfo *whreceiver.WebhookPostInfo,
) {
	logger.Debug("Recieved a webhook post")
	actions := filterOutAction(project, webhookInfo)
	if len(actions) == 0 {
		logger.Info("No applicable actions found in webhook post")
	}
	for _, action := range actions {
		pipeId := uuid.NewString()
		pipeLogger := logger.With(slog.String("pipeId", pipeId))
		streams, err := GetActionIoStreams(actions_output_dir, pipeId, logger)
		if err != nil {
			logger.Error("Error creating action's IO streams", slog.Any("error", err))
			continue
		}
		defer streams.Close()
		if len(action.Run) > 0 {
			executeActionRun(pipeLogger.With(slog.Any("command", action.Run)), action, streams)
		} else {
			executeActionScript(logger, action, streams)
		}
	}
}

func filterOutAction(project *config.Project, webhookInfo *whreceiver.WebhookPostInfo) []config.Action {
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

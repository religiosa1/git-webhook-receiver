package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/religiosa1/webhook-receiver/internal/ActionRunner"
	"github.com/religiosa1/webhook-receiver/internal/config"
	"github.com/religiosa1/webhook-receiver/internal/whreceiver"
)

func HandleWebhookPost(
	actionsCh chan ActionRunner.ActionArgs,
	logger *slog.Logger,
	cfg config.Config,
	project config.Project,
	receiver whreceiver.Receiver,
) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// setting nop-closer body, so we can read it multiple times
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			logger.Error("Error while reading the POST request body", slog.Any("error", err))
		}
		whReq := whreceiver.WebhookPostRequest{Payload: payload, Headers: req.Header}

		webhookInfo, err := receiver.GetWebhookInfo(whReq)
		if err != nil {
			errInfo := getWebhookErrorCode(err)
			logger.Error("Error while parsing the webhook request", slog.String("error", errInfo.Message))
			w.WriteHeader(errInfo.StatusCode)
			w.Write([]byte("{err}"))
			return
		}
		deliveryLogger := logger.With(slog.String("delivery", webhookInfo.DeliveryID))
		deliveryLogger.Info("Recieved a webhook post", slog.Any("webhookInfo", webhookInfo))

		if project.Authorization != "" {
			if authed, err := receiver.Authorize(whReq, project.Authorization); err != nil || !authed {
				deliveryLogger.Warn("Request authentications failed", slog.Any("error", err))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}

		if project.Secret != "" {
			if verified, err := receiver.VerifySignature(whReq, project.Secret); err != nil || !verified {
				deliveryLogger.Warn("Request signature is not valid", slog.Any("error", err))
				w.WriteHeader(http.StatusForbidden)
				return
			}
		}

		if receiver.IsPingRequest(whReq) {
			w.WriteHeader(http.StatusOK)
			return
		}

		actions := getProjectsActionsForWebhookPost(project, webhookInfo)
		if len(actions) == 0 {
			deliveryLogger.Info("No applicable actions found in webhook post")
			w.WriteHeader(http.StatusOK)
			return
		}

		for _, actionDescriptor := range actions {
			actionsCh <- ActionRunner.ActionArgs{Logger: deliveryLogger, Action: actionDescriptor}
		}

		deliveryLogger.Info("Launched actions", slog.Any("actions", actions))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(actions)
	}
}

type ErrorInfo struct {
	StatusCode int
	Message    string
}

func getWebhookErrorCode(err error) ErrorInfo {
	if terr, ok := err.(whreceiver.IncorrectRepoError); ok {
		return ErrorInfo{http.StatusUnprocessableEntity, terr.Error()}
	}
	if terr, ok := err.(whreceiver.AuthorizationError); ok {
		return ErrorInfo{http.StatusForbidden, terr.Error()}
	}
	if errors.Is(err, io.EOF) {
		return ErrorInfo{http.StatusUnprocessableEntity, "Empty body supplied in the webhook request"}
	}
	return ErrorInfo{http.StatusBadRequest, err.Error()}
}

func getProjectsActionsForWebhookPost(project config.Project, webhookInfo *whreceiver.WebhookPostInfo) []ActionRunner.ActionDescriptor {
	actions := make([]ActionRunner.ActionDescriptor, 0)
	for index, action := range project.Actions {
		if action.Branch != webhookInfo.Branch {
			continue
		}
		if action.On != "*" && action.On != webhookInfo.Event {
			continue
		}
		actions = append(actions, ActionRunner.ActionDescriptor{
			ActionIdentifier: ActionRunner.ActionIdentifier{Index: index, PipeId: uuid.NewString()},
			Action:           action,
		})
	}
	return actions
}

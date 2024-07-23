package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/religiosa1/webhook-receiver/internal/action_runner"
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
			errInfo := getWebhookErrorCode(err)
			logger.Error("Error while parsing the webhook request", slog.String("error", errInfo.Message))
			w.WriteHeader(errInfo.StatusCode)
			return
		}
		deliveryLogger := logger.With(slog.String("delivery", webhookInfo.DeliveryID))
		deliveryLogger.Debug("Recieved a webhook post", slog.Any("webhookInfo", webhookInfo))

		actions := getProjectsActionsForWebhookPost(project, webhookInfo)
		if len(actions) == 0 {
			deliveryLogger.Info("No applicable actions found in webhook post")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		authorizationResult := authorizeActions(*webhookInfo, actions)
		switch len(authorizationResult.Ok) {
		case len(actions):
			w.WriteHeader(http.StatusOK)
		case 0:
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusMultiStatus)
		}
		if len(authorizationResult.Forbidden) > 0 {
			deliveryLogger.Warn("Incorrect authorization information passed for actions",
				slog.String("auth", webhookInfo.Auth),
				slog.Any("actions", action_runner.GetActionIds(authorizationResult.Forbidden)),
			)
		}
		if len(authorizationResult.Ok) > 0 {
			go action_runner.ExecuteActions(deliveryLogger, authorizationResult.Ok, cfg.ActionsOutputDir, cfg.MaxOutputFiles)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(authorizationResultToWebhookPostResult(authorizationResult))
	}
}

type WebhookPostResult struct {
	Ok        []string `json:"ok,omitempty"`
	Forbidden []string `json:"forbidden,omitempty"`
}

func authorizationResultToWebhookPostResult(authResult ActionAuthorizationResult) WebhookPostResult {
	result := WebhookPostResult{
		Ok:        make([]string, len(authResult.Ok)),
		Forbidden: make([]string, len(authResult.Forbidden)),
	}
	for i := 0; i < len(authResult.Ok); i++ {
		result.Ok[i] = authResult.Ok[i].PipeId
	}
	for i := 0; i < len(authResult.Forbidden); i++ {
		result.Forbidden[i] = authResult.Forbidden[i].PipeId
	}
	return result
}

type ActionAuthorizationResult struct {
	Ok        []action_runner.ActionDescriptor
	Forbidden []action_runner.ActionDescriptor
}

func authorizeActions(
	webhookInfo whreceiver.WebhookPostInfo,
	actions []action_runner.ActionDescriptor,
) ActionAuthorizationResult {
	result := ActionAuthorizationResult{}
	for _, actionDesc := range actions {
		if actionDesc.Action.Authorization != "" && actionDesc.Action.Authorization != webhookInfo.Auth {
			result.Forbidden = append(result.Forbidden, actionDesc)
			continue
		}
		result.Ok = append(result.Ok, actionDesc)
	}
	return result
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

func getProjectsActionsForWebhookPost(project *config.Project, webhookInfo *whreceiver.WebhookPostInfo) []action_runner.ActionDescriptor {
	actions := make([]action_runner.ActionDescriptor, 0)
	for index, action := range project.Actions {
		if action.Branch != webhookInfo.Branch {
			continue
		}
		if action.On != "*" && action.On != webhookInfo.Event {
			continue
		}
		actions = append(actions, action_runner.ActionDescriptor{
			ActionIdentifier: action_runner.ActionIdentifier{Index: index, PipeId: uuid.NewString()},
			Action:           action,
		})
	}
	return actions
}

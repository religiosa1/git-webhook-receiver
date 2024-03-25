package handlers

import (
	"encoding/json"
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
			errInfo := getWebhookErrorCode(err)
			logger.Error("Error while parsing the webhook request", slog.String("error", errInfo.Message))
			w.WriteHeader(errInfo.StatusCode)
			return
		}
		deliveryLogger := logger.With(slog.String("delivery", webhookInfo.DeliveryID))
		deliveryLogger.Debug("Recieved a webhook post", slog.Any("webhookInfo", webhookInfo))

		actions := filterOutAction(project, webhookInfo)
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
				slog.Any("actions", getActionIds(authorizationResult.Forbidden)),
			)
		}
		if len(authorizationResult.Ok) > 0 {
			go processWebHookPost(deliveryLogger, authorizationResult.Ok, cfg.ActionsOutputDir)
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
	Ok        []ActionDescriptor
	Forbidden []ActionDescriptor
}

func authorizeActions(
	webhookInfo whreceiver.WebhookPostInfo,
	actions []ActionDescriptor,
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

type ActionIdentifier struct {
	Index  int
	PipeId string
}
type ActionDescriptor struct {
	ActionIdentifier
	Action config.Action
}

func filterOutAction(project *config.Project, webhookInfo *whreceiver.WebhookPostInfo) []ActionDescriptor {
	actions := make([]ActionDescriptor, 0)
	for index, action := range project.Actions {
		if action.Branch != webhookInfo.Branch {
			continue
		}
		if action.On != "*" && action.On != webhookInfo.Event {
			continue
		}
		actions = append(actions, ActionDescriptor{ActionIdentifier{index, uuid.NewString()}, action})
	}
	return actions
}

func getActionIds(descs []ActionDescriptor) []ActionIdentifier {
	result := make([]ActionIdentifier, len(descs))
	for i := 0; i < len(descs); i++ {
		result[i] = descs[i].ActionIdentifier
	}
	return result
}

func processWebHookPost(
	logger *slog.Logger,
	actionDescriptors []ActionDescriptor,
	actions_output_dir string,
) {
	for _, actionDescriptor := range actionDescriptors {
		pipeLogger := logger.With(slog.String("pipeId", actionDescriptor.PipeId))
		pipeLogger.Info("Running action", slog.Int("action_index", actionDescriptor.Index))
		streams, err := GetActionIoStreams(actions_output_dir, actionDescriptor.PipeId, logger)
		if err != nil {
			logger.Error("Error creating action's IO streams", slog.Any("error", err))
			continue
		}
		defer streams.Close()
		if len(actionDescriptor.Action.Run) > 0 {
			executeActionRun(pipeLogger.With(slog.Any("command", actionDescriptor.Action.Run)), actionDescriptor.Action, streams)
		} else {
			executeActionScript(logger, actionDescriptor.Action, streams)
		}
	}
}

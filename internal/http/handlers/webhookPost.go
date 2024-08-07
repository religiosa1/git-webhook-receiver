package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/religiosa1/webhook-receiver/internal/action_runner"
	"github.com/religiosa1/webhook-receiver/internal/config"
	"github.com/religiosa1/webhook-receiver/internal/whreceiver"
)

func HandleWebhookPost(
	actionsCtx context.Context,
	actionsWg *sync.WaitGroup,
	logger *slog.Logger,
	cfg *config.Config,
	project *config.Project,
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
			return
		}
		deliveryLogger := logger.With(slog.String("delivery", webhookInfo.DeliveryID))
		deliveryLogger.Info("Recieved a webhook post", slog.Any("webhookInfo", webhookInfo))

		actions := getProjectsActionsForWebhookPost(project, webhookInfo)
		if len(actions) == 0 {
			deliveryLogger.Info("No applicable actions found in webhook post")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		authorizationResult := authorizeActions(deliveryLogger, receiver, whReq, actions)
		switch len(authorizationResult.Ok) {
		case len(actions):
			w.WriteHeader(http.StatusOK)
		case 0:
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusMultiStatus)
		}
		actionsWg.Add(len(authorizationResult.Ok))
		for _, actionDescriptor := range authorizationResult.Ok {
			go func() {
				defer actionsWg.Done()
				action_runner.ExecuteAction(actionsCtx, deliveryLogger, actionDescriptor, cfg.ActionsOutputDir)
			}()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(authorizationResultToWebhookPostResult(authorizationResult))
	}
}

type WebhookPostResult struct {
	Ok           []string `json:"ok,omitempty"`
	Forbidden    []string `json:"forbidden,omitempty"`
	BadSignature []string `json:"badSignature,omitempty"`
}

func authorizationResultToWebhookPostResult(authResult ActionAuthorizationResult) WebhookPostResult {
	result := WebhookPostResult{
		Ok:           make([]string, len(authResult.Ok)),
		Forbidden:    make([]string, len(authResult.Forbidden)),
		BadSignature: make([]string, len(authResult.BadSignature)),
	}
	for i := 0; i < len(authResult.Ok); i++ {
		result.Ok[i] = authResult.Ok[i].PipeId
	}
	for i := 0; i < len(authResult.Forbidden); i++ {
		result.Forbidden[i] = authResult.Forbidden[i].PipeId
	}
	for i := 0; i < len(authResult.BadSignature); i++ {
		result.BadSignature[i] = authResult.BadSignature[i].PipeId
	}
	return result
}

type ActionAuthorizationResult struct {
	Ok           []action_runner.ActionDescriptor
	Forbidden    []action_runner.ActionDescriptor
	BadSignature []action_runner.ActionDescriptor
}

func authorizeActions(
	logger *slog.Logger,
	receiver whreceiver.Receiver,
	req whreceiver.WebhookPostRequest,
	actions []action_runner.ActionDescriptor,
) ActionAuthorizationResult {
	result := ActionAuthorizationResult{}
	for _, actionDesc := range actions {
		if auth := actionDesc.Action.Authorization; auth != "" {
			authed, err := receiver.Authorize(req, auth)
			// We're not logging secrets or auth data for security reasons.
			if err != nil || !authed {
				if err != nil {
					logger.Error("Error during the signature validation", slog.Any("error", err))
				} else {
					logger.Warn("Incorrect authorization information passed for action",
						slog.Any("action", actionDesc.ActionIdentifier),
					)
				}
				result.Forbidden = append(result.Forbidden, actionDesc)
				continue
			}
		}
		if secret := actionDesc.Action.Secret; secret != "" {
			valid, err := receiver.VerifySignature(req, secret)
			if err != nil || !valid {
				if err != nil {
					logger.Error("Error during the signature validation", slog.Any("error", err))
				} else {
					logger.Warn("Incorrect secret signature provided for actions",
						slog.Any("action", actionDesc.ActionIdentifier),
					)
				}
				result.BadSignature = append(result.BadSignature, actionDesc)
				continue
			}
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

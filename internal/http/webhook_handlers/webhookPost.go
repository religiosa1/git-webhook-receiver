package webhookhandlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/oklog/ulid/v2"
	"github.com/religiosa1/git-webhook-receiver/internal/ActionRunner"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
	"github.com/religiosa1/git-webhook-receiver/internal/http/utils"
	"github.com/religiosa1/git-webhook-receiver/internal/whreceiver"
)

// 300 KiB max body size
const maxBodySize int64 = 1024 * 300

func HandleWebhookPost(
	actionsCh chan ActionRunner.ActionArgs,
	logger *slog.Logger,
	cfg config.Config,
	projectName string,
	project config.Project,
	receiver whreceiver.Receiver,
) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		req.Body = http.MaxBytesReader(w, req.Body, maxBodySize)
		// setting noop-closer body, so we can read it multiple times
		payload, err := io.ReadAll(req.Body)
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		if err != nil {
			logger.Error("Error while reading the POST request body", slog.Any("error", err))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		whReq := whreceiver.WebhookPostRequest{Payload: payload, Headers: req.Header}

		webhookInfo, err := receiver.GetWebhookInfo(whReq)
		if err != nil {
			errInfo := getWebhookErrorCode(err)
			logger.Error("Error while parsing the webhook request", slog.String("error", errInfo.Message))
			if writeErr := utils.WriteErrorResponse(w, errInfo.StatusCode, errInfo.Message); writeErr != nil {
				logger.Error("error while writing error message", slog.Any("error", writeErr))
			}
			return
		}
		deliveryLogger := logger.With(slog.String("deliveryId", webhookInfo.DeliveryID))
		deliveryLogger.Info("Received a webhook post", slog.Any("webhookInfo", webhookInfo))
		if webhookInfo.Branch == "" {
			deliveryLogger.Info("no branch name captured out of the payload ref")
		}

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

		actions := getProjectsActionsForWebhookPost(projectName, project, webhookInfo)
		if len(actions) == 0 {
			deliveryLogger.Info("No applicable actions found in webhook post")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		for _, actionDescriptor := range actions {
			actionsCh <- ActionRunner.ActionArgs{Logger: deliveryLogger, Action: actionDescriptor}
		}

		deliveryLogger.Info("Launched actions", slog.Any("actions", actions))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		err = json.NewEncoder(w).Encode(actionsToOutput(cfg, actions))
		if err != nil {
			deliveryLogger.Error("Error while encoding action's output", slog.Any("error", err))
		}
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

func getProjectsActionsForWebhookPost(projectName string, project config.Project, webhookInfo *whreceiver.WebhookPostInfo) []ActionRunner.ActionDescriptor {
	actions := make([]ActionRunner.ActionDescriptor, 0)
	for index, action := range project.Actions {
		if action.Branch != "*" && action.Branch != webhookInfo.Branch {
			continue
		}
		if action.On != "*" && action.On != webhookInfo.Event {
			continue
		}
		actions = append(actions, ActionRunner.ActionDescriptor{
			ActionIdentifier: ActionRunner.ActionIdentifier{
				Index:      index,
				PipeID:     ulid.Make().String(),
				Project:    projectName,
				DeliveryID: webhookInfo.DeliveryID,
			},
			Action: action,
		})
	}
	return actions
}

type ActionOutput struct {
	ActionRunner.ActionIdentifier
	URL string `json:"url,omitempty"`
}

func actionsToOutput(cfg config.Config, actions []ActionRunner.ActionDescriptor) []ActionOutput {
	output := make([]ActionOutput, len(actions))
	baseURL := generatePublicBaseURL(cfg)
	for idx, action := range actions {
		var url string
		if baseURL != "" {
			url = baseURL + action.PipeID
		}
		output[idx] = ActionOutput{
			ActionIdentifier: action.ActionIdentifier,
			URL:              url,
		}
	}
	return output
}

func generatePublicBaseURL(cfg config.Config) string {
	if cfg.DisableAPI {
		return ""
	}
	if cfg.PublicURL != "" {
		return strings.TrimSuffix(cfg.PublicURL, "/") + "/pipelines/"
	}

	protocol := "http"
	if cfg.Ssl.CertFilePath != "" && cfg.Ssl.KeyFilePath != "" {
		protocol = "https"
	}

	return fmt.Sprintf("%s://%s/pipelines/", protocol, cfg.Addr)
}

package webhookhandlers

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/oklog/ulid/v2"
	"github.com/religiosa1/git-webhook-receiver/internal/ActionRunner"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
	"github.com/religiosa1/git-webhook-receiver/internal/http/utils"
	"github.com/religiosa1/git-webhook-receiver/internal/whreceiver"
)

// 300 KiB max body size
const maxBodySize int64 = 1024 * 300

type Webhook struct {
	ActionsCh   chan ActionRunner.ActionArgs
	Config      config.Config
	ProjectName string
	Project     config.Project
	Receiver    whreceiver.Receiver
}

func (h Webhook) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger := middleware.GetLogger(req.Context())
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

	webhookInfo, err := h.Receiver.GetWebhookInfo(whReq)
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

	if h.Project.Authorization != "" {
		if authed, err := h.Receiver.Authorize(whReq, h.Project.Authorization); err != nil || !authed {
			deliveryLogger.Warn("Request authentications failed", slog.Any("error", err))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	if h.Project.Secret != "" {
		if verified, err := h.Receiver.VerifySignature(whReq, h.Project.Secret); err != nil || !verified {
			deliveryLogger.Warn("Request signature is not valid", slog.Any("error", err))
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}

	if h.Receiver.IsPingRequest(whReq) {
		w.WriteHeader(http.StatusOK)
		return
	}

	actions := getProjectsActionsForWebhookPost(h.ProjectName, h.Project, webhookInfo)
	if len(actions) == 0 {
		deliveryLogger.Info("No applicable actions found in webhook post")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	for _, actionDescriptor := range actions {
		h.ActionsCh <- ActionRunner.ActionArgs{Logger: deliveryLogger, Action: actionDescriptor}
	}

	deliveryLogger.Info("Launched actions", slog.Any("actions", actions))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(actionsToOutput(h.Config, actions))
	if err != nil {
		deliveryLogger.Error("Error while encoding action's output", slog.Any("error", err))
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
	Links *ActionLinks `json:"links,omitempty"`
}

func actionsToOutput(cfg config.Config, actions []ActionRunner.ActionDescriptor) []ActionOutput {
	output := make([]ActionOutput, len(actions))
	for idx, action := range actions {
		output[idx] = ActionOutput{
			ActionIdentifier: action.ActionIdentifier,
			Links:            generateLinks(cfg.DisableAPI, cfg.PublicURL, action.PipeID),
		}
	}
	return output
}

type ActionLinks struct {
	Details string `json:"details"`
	Output  string `json:"output"`
}

func generateLinks(disableApi bool, publicURL string, pipeID string) *ActionLinks {
	if disableApi || publicURL == "" {
		return nil
	}
	pipeID = url.PathEscape(pipeID)
	details, err := url.JoinPath(publicURL, "pipelines", pipeID)
	if err != nil {
		return nil
	}
	output, err := url.JoinPath(publicURL, "pipelines", pipeID, "output")
	if err != nil {
		return nil
	}
	return &ActionLinks{
		Details: details,
		Output:  output,
	}
}

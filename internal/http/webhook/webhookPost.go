package webhook

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/oklog/ulid/v2"
	"github.com/religiosa1/git-webhook-receiver/internal/actionrunner"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
	"github.com/religiosa1/git-webhook-receiver/internal/http/utils"
	"github.com/religiosa1/git-webhook-receiver/internal/whreceiver"
)

// 300 KiB max body size
const maxBodySize int64 = 1024 * 300

type Webhook struct {
	ActionsCh   chan<- actionrunner.ActionArgs
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

	if !h.Project.Authorization.IsZero() {
		if authed, err := h.Receiver.Authorize(whReq, h.Project.Authorization.RawContents()); err != nil || !authed {
			deliveryLogger.Warn("Request authentications failed", slog.Any("error", err))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	if !h.Project.Secret.IsZero() {
		if verified, err := h.Receiver.VerifySignature(whReq, h.Project.Secret.RawContents()); err != nil || !verified {
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

	for _, actionDesc := range actions {
		actionLogger := deliveryLogger.With(slog.Any("action", actionDesc.ActionIdentifier))
		args := actionrunner.ActionArgs{
			Logger:     actionLogger,
			ActionDesc: actionDesc,
			DeliveryID: webhookInfo.DeliveryID,
			Hash:       webhookInfo.Hash,
		}
		select {
		case h.ActionsCh <- args:
			actionLogger.Info("Launched action")
		default:
			actionLogger.Error("Unable to queue the action, as action runner is at full queue capacity")
			w.WriteHeader(http.StatusTooManyRequests)
			// we're not accounting for partial success here, returning 429 on any blockage.
			// The idea is -- there will be a retry; trade-off is that successful actions
			// will run twice on retries, than failed ones
			return
		}
	}

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

func getProjectsActionsForWebhookPost(
	projectName string,
	project config.Project,
	webhookInfo *whreceiver.WebhookPostInfo,
) []actionrunner.ActionDescriptor {
	actions := make([]actionrunner.ActionDescriptor, 0)
	for index, action := range project.Actions {
		if action.Branch != "*" && action.Branch != webhookInfo.Branch {
			continue
		}
		if action.On != "*" && action.On != webhookInfo.Event {
			continue
		}
		actions = append(actions, actionrunner.ActionDescriptor{
			ActionIdentifier: actionrunner.ActionIdentifier{
				Index:   index,
				PipeID:  ulid.Make().String(),
				Project: projectName,
			},
			Config: action,
		})
	}
	return actions
}

type ActionOutput struct {
	actionrunner.ActionIdentifier
	Links *ActionLinks `json:"links,omitempty"`
}

func actionsToOutput(cfg config.Config, actions []actionrunner.ActionDescriptor) []ActionOutput {
	output := make([]ActionOutput, len(actions))
	for idx, action := range actions {
		output[idx] = ActionOutput{
			ActionIdentifier: action.ActionIdentifier,
			Links:            generateLinks(determineLinksType(cfg), cfg.PublicURL, action.PipeID),
		}
	}
	return output
}

type linksTypeEnum int

const (
	linksTypeNone linksTypeEnum = 0
	linksTypeUI   linksTypeEnum = 1
	linksTypeAPI  linksTypeEnum = 2
)

func determineLinksType(cfg config.Config) linksTypeEnum {
	switch {
	case !cfg.DisableUI && cfg.ActionsDBFile != "":
		return linksTypeUI
	case !cfg.DisableAPI && cfg.ActionsDBFile != "":
		return linksTypeAPI
	default:
		return linksTypeNone
	}
}

type ActionLinks struct {
	Details string `json:"details"`
	Output  string `json:"output"`
}

func generateLinks(linksType linksTypeEnum, publicURL string, pipeID string) *ActionLinks {
	if linksType == linksTypeNone || publicURL == "" {
		return nil
	}
	pipeID = url.PathEscape(pipeID)
	var detailsElem []string
	var outputElem []string
	switch linksType {
	case linksTypeUI:
		detailsElem = []string{"pipelines", pipeID}
		outputElem = []string{"pipelines", pipeID, "output"}
	case linksTypeAPI:
		detailsElem = []string{"api", "pipelines", pipeID}
		outputElem = []string{"api", "pipelines", pipeID, "output"}
	}
	details, err := url.JoinPath(publicURL, detailsElem...)
	if err != nil {
		return nil
	}
	output, err := url.JoinPath(publicURL, outputElem...)
	if err != nil {
		return nil
	}
	return &ActionLinks{
		Details: details,
		Output:  output,
	}
}

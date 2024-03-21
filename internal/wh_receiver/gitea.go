package wh_receiver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/religiosa1/deployer/internal/config"
)

type GiteaReceiver struct {
	project *config.Project
}

func (receiver GiteaReceiver) GetWebhookInfo(req *http.Request) (*WebhookPostInfo, error) {
	// TODO check authorization headers
	var payload GiteaWebhookPayload
	err := json.NewDecoder(req.Body).Decode(&payload)
	if err != nil {
		return nil, err
	}
	if payload.Repository.FullName != receiver.project.Repo {
		return nil, IncorrectRepoError{Expected: receiver.project.Repo, Actual: payload.Repository.FullName}
	}
	branch := getBranchFromRefName(payload.Ref)
	hash := payload.After
	event := req.Header.Get("x-gitea-event")

	return &WebhookPostInfo{branch, event, hash}, nil
}

func getBranchFromRefName(ref string) string {
	parts := strings.Split(ref, "/")
	if len(parts) < 3 {
		return ref
	}
	branchName := strings.Join(parts[2:], "/")
	return branchName
}

type GiteaWebhookPayload struct {
	Ref        string           `json:"ref"`
	After      string           `json:"after"`
	Repository GiteaWebhookRepo `json:"repository"`
}

type GiteaWebhookRepo struct {
	FullName string `json:"full_name"`
}

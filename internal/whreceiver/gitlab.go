package whreceiver

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

type GitlabReceiver struct {
	project config.Project
}

func (rcvr GitlabReceiver) GetCapabilities() ReceiverCapabilities {
	return ReceiverCapabilities{
		CanAuthorize:       true,
		CanVerifySignature: false,
		HasPing:            false,
	}
}

func (rcvr GitlabReceiver) Authorize(req WebhookPostRequest, auth string) (bool, error) {
	authorizationHeader := req.Headers.Get("X-Gitlab-Token")
	isSame := subtle.ConstantTimeCompare([]byte(auth), []byte(authorizationHeader)) == 1
	return isSame, nil
}

// https://gitlab.com/gitlab-org/gitlab/-/issues/19367
func (rcvr GitlabReceiver) VerifySignature(req WebhookPostRequest, secret string) (bool, error) {
	return false, ErrSignNotSupported
}

func (rcvr GitlabReceiver) IsPingRequest(req WebhookPostRequest) bool {
	return false
}

var gitlabEventSuffix = " Hook"

type gitlabWebhookPayload struct {
	Ref     string `json:"ref"`
	After   string `json:"after"`
	Project struct {
		PathWithNamespace string `json:"path_with_namespace"`
	} `json:"project"`
}

func (rcvr GitlabReceiver) GetWebhookInfo(req WebhookPostRequest) (*WebhookPostInfo, error) {
	var postInfo WebhookPostInfo
	var whPayload gitlabWebhookPayload
	if err := json.NewDecoder(bytes.NewBuffer(req.Payload)).Decode(&whPayload); err != nil {
		return nil, err
	}
	repo := whPayload.Project.PathWithNamespace
	if repo != rcvr.project.Repo {
		return nil, IncorrectRepoError{Expected: rcvr.project.Repo, Actual: repo}
	}

	postInfo.Branch = getBranchFromRefName(whPayload.Ref)

	postInfo.Hash = whPayload.After
	event := req.Headers.Get("X-Gitlab-Event")

	if !strings.HasSuffix(event, gitlabEventSuffix) {
		return nil, fmt.Errorf("malformed gitlab event, must end with ' Hook', got %s", event)
	}
	postInfo.Event = strings.ToLower(event[:len(event)-len(gitlabEventSuffix)])
	postInfo.DeliveryID = req.Headers.Get("X-Gitlab-Event-UUID")
	return &postInfo, nil
}

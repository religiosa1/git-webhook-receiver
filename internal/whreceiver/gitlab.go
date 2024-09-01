package whreceiver

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"

	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

type GitlabReceiver struct {
	project config.Project
}

func (rcvr GitlabReceiver) Authorize(req WebhookPostRequest, auth string) (bool, error) {
	authorizationHeader := req.Headers.Get("X-Gitlab-Token")
	isSame := subtle.ConstantTimeCompare([]byte(auth), []byte(authorizationHeader)) == 1
	return isSame, nil
}

// https://gitlab.com/gitlab-org/gitlab/-/issues/19367
func (rcvr GitlabReceiver) VerifySignature(req WebhookPostRequest, secret string) (bool, error) {
	return false, errors.New("request signature is not supported for gitlab receiver, use Authorize instead")
}

func (rcvr GitlabReceiver) IsPingRequest(req WebhookPostRequest) bool {
	return false
}

var gitlabEventSuffix = " Hook"

func (rcvr GitlabReceiver) GetWebhookInfo(req WebhookPostRequest) (*WebhookPostInfo, error) {
	postInfo, err := getJsonPayloadInfo(req.Payload, rcvr.project.Repo)
	if err != nil {
		return nil, err
	}
	event := req.Headers.Get("X-Gitlab-Event")

	if !strings.HasSuffix(event, gitlabEventSuffix) {
		return nil, fmt.Errorf("malformed gitlab event, must end with ' Hook', got %s", event)
	}
	postInfo.Event = event[:len(event)-len(gitlabEventSuffix)]
	postInfo.DeliveryID = req.Headers.Get("X-Gitlab-Event-UUID")
	return postInfo, nil
}

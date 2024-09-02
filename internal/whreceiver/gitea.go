package whreceiver

import (
	"crypto/subtle"

	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

type GiteaReceiver struct {
	project config.Project
}

func (rcvr GiteaReceiver) GetCapabilities() ReceiverCapabilities {
	return ReceiverCapabilities{
		CanAuthorize:       true,
		CanVerifySignature: true,
		HasPing:            false,
	}
}

func (rcvr GiteaReceiver) Authorize(req WebhookPostRequest, auth string) (bool, error) {
	authorizationHeader := req.Headers.Get("Authorization")
	isSame := subtle.ConstantTimeCompare([]byte(auth), []byte(authorizationHeader)) == 1
	return isSame, nil
}

func (rcvr GiteaReceiver) VerifySignature(req WebhookPostRequest, secret string) (bool, error) {
	signature := req.Headers.Get("X-Gitea-Signature")
	if signature == "" {
		return false, nil
	}
	return verifyPayloadSignature(req.Payload, signature, secret)
}

func (rcvr GiteaReceiver) GetWebhookInfo(req WebhookPostRequest) (*WebhookPostInfo, error) {
	postInfo, err := getJsonPayloadInfo(req.Payload, rcvr.project.Repo)
	if err != nil {
		return nil, err
	}
	postInfo.Event = req.Headers.Get("X-Gitea-Event")
	postInfo.DeliveryID = req.Headers.Get("X-Gitea-Delivery")

	return postInfo, nil
}

func (rcvr GiteaReceiver) IsPingRequest(req WebhookPostRequest) bool {
	return false
}

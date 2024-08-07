package whreceiver

import (
	"errors"
	"fmt"
	"strings"

	"github.com/religiosa1/webhook-receiver/internal/config"
)

type GithubReceiver struct {
	project *config.Project
}

func (rcvr GithubReceiver) Authorize(req WebhookPostRequest, auth string) (bool, error) {
	return false, errors.New("authorization header is not supported for github receiver, use secret signature instead")
}

const signaturePrefix = "sha256="

func (rcvr GithubReceiver) VerifySignature(req WebhookPostRequest, secret string) (bool, error) {
	signature := req.Headers.Get("X-Hub-Signature-256")
	if signature == "" || signature == signaturePrefix {
		return false, nil
	}
	if !strings.HasPrefix(signature, signaturePrefix) {
		return false, fmt.Errorf("malformed GitHub signature: it must start with '"+signaturePrefix+"', got %s instead", signature)
	}
	signature = signature[len(signaturePrefix):]
	return verifyPayloadSignature(req.Payload, signature, secret)
}

func (rcvr GithubReceiver) GetWebhookInfo(req WebhookPostRequest) (*WebhookPostInfo, error) {
	postInfo, err := getJsonPayloadInfo(req.Payload, rcvr.project.Repo)
	if err != nil {
		return nil, err
	}
	postInfo.Event = req.Headers.Get("X-GitHub-Event")
	postInfo.DeliveryID = req.Headers.Get("X-GitHub-Delivery")
	return postInfo, nil
}

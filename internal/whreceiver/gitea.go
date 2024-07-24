package whreceiver

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/religiosa1/webhook-receiver/internal/config"
)

type GiteaReceiver struct {
	project *config.Project
}

func (receiver GiteaReceiver) GetWebhookInfo(req WebhookPostRequest) (*WebhookPostInfo, error) {
	var whPayload GiteaWebhookPayload
	err := json.NewDecoder(bytes.NewBuffer(req.Payload)).Decode(&whPayload)
	if err != nil {
		return nil, err
	}
	if whPayload.Repository.FullName != receiver.project.Repo {
		return nil, IncorrectRepoError{Expected: receiver.project.Repo, Actual: whPayload.Repository.FullName}
	}
	branch := getBranchFromRefName(whPayload.Ref)
	hash := whPayload.After
	event := req.Headers.Get("x-gitea-event")
	deliveryID := req.Headers.Get("X-Gitea-Delivery")

	return &WebhookPostInfo{
		deliveryID,
		branch,
		event,
		hash,
	}, nil
}

func (receiver GiteaReceiver) Authorize(req WebhookPostRequest, auth string) (bool, error) {
	authorizationHeader := req.Headers.Get("Authorization")
	isSame := subtle.ConstantTimeCompare([]byte(auth), []byte(authorizationHeader)) == 1
	return isSame, nil
}

func (receiver GiteaReceiver) ValidateSignature(req WebhookPostRequest, secret string) (bool, error) {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(req.Payload)
	payloadSignature := h.Sum(nil)
	headerSignatureString := req.Headers.Get("X-Gitea-Signature")
	if headerSignatureString == "" {
		return false, nil
	}
	headerSignature, err := hex.DecodeString(headerSignatureString)
	if err != nil {
		return false, err
	}

	signatureMatch := subtle.ConstantTimeCompare(payloadSignature, headerSignature) == 1

	return signatureMatch, nil
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

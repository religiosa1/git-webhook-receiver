package whreceiver

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/religiosa1/git-webhook-receiver/internal/cryptoutils"
)

// CommonWebhookPayload is common significant payload for push events for Gitea and Github
type CommonWebhookPayload struct {
	Ref        string            `json:"ref"`
	After      string            `json:"after"`
	Repository CommonWebhookRepo `json:"repository"`
}

type CommonWebhookRepo struct {
	FullName string `json:"full_name"`
}

// Returns partial data from the payload common for gitea and github, to be populated
// with the headers information down the line.
func getJSONPayloadInfo(payload []byte, repo string) (*WebhookPostInfo, error) {
	var whPayload CommonWebhookPayload
	if err := json.Unmarshal(payload, &whPayload); err != nil {
		return nil, err
	}
	if whPayload.Repository.FullName != repo {
		return nil, IncorrectRepoError{Expected: repo, Actual: whPayload.Repository.FullName}
	}

	branch := getBranchFromRefName(whPayload.Ref)
	hash := whPayload.After
	return &WebhookPostInfo{Branch: branch, Hash: hash}, nil
}

func verifyPayloadSignature(payload []byte, signature string, secret string) (bool, error) {
	headSig, err := hex.DecodeString(signature)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature: %w", err)
	}

	payloadSignature := getPayloadSignature(secret, payload)

	return cryptoutils.NewConstantTimeComparerBytes(headSig).EqBytes((payloadSignature)), nil
}

func getBranchFromRefName(ref string) string {
	parts := strings.Split(ref, "/")
	var refType, refName string
	if len(parts) >= 2 {
		refType = parts[1]
	}
	if len(parts) >= 3 {
		refName = strings.Join(parts[2:], "/")
	}
	if refType != "heads" {
		return ""
	}
	return refName
}

func getPayloadSignature(secret string, payload []byte) []byte {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	payloadSignature := h.Sum(nil)

	return payloadSignature
}

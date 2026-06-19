package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/religiosa1/git-webhook-receiver/internal/http/api"
	"github.com/religiosa1/git-webhook-receiver/internal/tmpoutput"
)

func TestGetPipelineOutput(t *testing.T) {
	db := newTestActionDB(t)
	pipeID := ulid.Make().String()
	const outputText = "line one\nline two\n"
	seedActionDBCompletedRecord(t, db, pipeID, "proj", "abc1234", "del", outputText, nil)

	handler := api.GetPipelineOutput{DB: db, TmpOutputMgr: tmpoutput.NewInMemoryTmpOutput(0)}

	t.Run("returns 200 with plain text output", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/pipelines/"+pipeID+"/output", nil)
		req.SetPathValue("pipeId", pipeID)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := rec.Code; got != http.StatusOK {
			t.Errorf("status: want %d, got %d", http.StatusOK, got)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
			t.Errorf("Content-Type: want text/plain, got %q", ct)
		}
		if got := rec.Body.String(); got != outputText {
			t.Errorf("body: want %q, got %q", outputText, got)
		}
	})

	t.Run("returns 404 for non-existent pipeId", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/pipelines/nosuchid/output", nil)
		req.SetPathValue("pipeId", "nosuchid")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := rec.Code; got != http.StatusNotFound {
			t.Errorf("status: want %d, got %d", http.StatusNotFound, got)
		}
	})

	t.Run("returns empty body for record with no output", func(t *testing.T) {
		recordID := ulid.Make().String()
		seedActionDBRecord(t, db, recordID, "proj", "abc1234", "del")

		req := httptest.NewRequest(http.MethodGet, "/pipelines/"+recordID+"/output", nil)
		req.SetPathValue("pipeId", recordID)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := rec.Code; got != http.StatusNoContent {
			t.Errorf("status: want %d, got %d", http.StatusNoContent, got)
		}
		if got := rec.Body.String(); got != "" {
			t.Errorf("body: want empty, got %q", got)
		}
	})
}

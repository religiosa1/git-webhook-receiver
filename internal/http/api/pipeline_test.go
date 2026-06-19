package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/religiosa1/git-webhook-receiver/internal/http/api"
)

func TestGetPipeline(t *testing.T) {
	db := newTestActionDB(t)
	pipeID := ulid.Make().String()
	seedActionDBCompletedRecord(t, db, pipeID, "myproject", "d3adb33f", "del-123", "hello output", nil)

	handler := api.GetPipeline{DB: db}

	t.Run("returns 200 with record data for existing pipeId", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/pipelines/"+pipeID, nil)
		req.SetPathValue("pipeId", pipeID)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := rec.Code; got != http.StatusOK {
			t.Errorf("status: want %d, got %d", http.StatusOK, got)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("Content-Type: want application/json, got %q", ct)
		}
		var resp itemResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.PipeID != pipeID {
			t.Errorf("pipeId: want %q, got %q", pipeID, resp.PipeID)
		}
		if resp.Project != "myproject" {
			t.Errorf("project: want %q, got %q", "myproject", resp.Project)
		}
		if resp.DeliveryID != "del-123" {
			t.Errorf("deliveryId: want %q, got %q", "del-123", resp.DeliveryID)
		}
	})

	t.Run("returns 404 for non-existent pipeId", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/pipelines/nosuchid", nil)
		req.SetPathValue("pipeId", "nosuchid")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := rec.Code; got != http.StatusNotFound {
			t.Errorf("status: want %d, got %d", http.StatusNotFound, got)
		}
	})
}

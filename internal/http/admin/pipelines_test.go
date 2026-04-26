package admin_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
	actiondb "github.com/religiosa1/git-webhook-receiver/internal/actionDb"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
	"github.com/religiosa1/git-webhook-receiver/internal/http/admin"
	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
)

// TODO: a lot of stuff here is testing repo-layer functionality. We need to move
// all of pagination and overlap tests to the ActionDb_test.go

var testAction = config.Action{
	On:     "push",
	Branch: "main",
	Script: "echo test",
}

func newTestDB(t *testing.T) *actiondb.ActionDB {
	t.Helper()
	db, err := actiondb.New(":memory:", 1000, 0)
	if err != nil {
		t.Fatalf("failed to create test DB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedRecord(t *testing.T, db *actiondb.ActionDB, pipeID, project, deliveryID string) {
	t.Helper()
	if err := db.CreateRecord(pipeID, project, deliveryID, testAction); err != nil {
		t.Fatalf("seed record %s: %v", pipeID, err)
	}
}

func seedCompletedRecord(t *testing.T, db *actiondb.ActionDB, pipeID, project, deliveryID, output string, cmdErr error) {
	t.Helper()
	seedRecord(t, db, pipeID, project, deliveryID)
	if err := db.CloseRecord(pipeID, cmdErr, strings.NewReader(output)); err != nil {
		t.Fatalf("close record %s: %v", pipeID, err)
	}
}

type listResponse struct {
	Items      []itemResponse `json:"items"`
	TotalCount int            `json:"totalCount"`
	NextPage   *string        `json:"nextPage"`
}

type itemResponse struct {
	PipeID     string `json:"pipeId"`
	Project    string `json:"project"`
	DeliveryID string `json:"deliveryId"`
}

func decodeListResponse(t *testing.T, body io.Reader) listResponse {
	t.Helper()
	var resp listResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	return resp
}

func collectPipeIDs(items []itemResponse) map[string]bool {
	ids := make(map[string]bool, len(items))
	for _, item := range items {
		ids[item.PipeID] = true
	}
	return ids
}

func TestListPipelines(t *testing.T) {
	t.Run("empty db returns empty items and zero total", func(t *testing.T) {
		db := newTestDB(t)
		handler := admin.ListPipelines{DB: db}

		req := httptest.NewRequest(http.MethodGet, "/pipelines", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := rec.Code; got != http.StatusOK {
			t.Errorf("status: want %d, got %d", http.StatusOK, got)
		}
		resp := decodeListResponse(t, rec.Body)
		if len(resp.Items) != 0 {
			t.Errorf("items: want empty, got %d", len(resp.Items))
		}
		if resp.TotalCount != 0 {
			t.Errorf("totalCount: want 0, got %d", resp.TotalCount)
		}
		if resp.NextPage != nil {
			t.Errorf("nextPage: want nil, got %q", *resp.NextPage)
		}
	})

	t.Run("returns correct content-type", func(t *testing.T) {
		db := newTestDB(t)
		handler := admin.ListPipelines{DB: db}

		req := httptest.NewRequest(http.MethodGet, "/pipelines", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("Content-Type: want application/json, got %q", ct)
		}
	})

	t.Run("returns all seeded records with correct totalCount", func(t *testing.T) {
		db := newTestDB(t)
		const n = 5
		for range n {
			seedRecord(t, db, ulid.Make().String(), "proj", "del")
		}

		handler := admin.ListPipelines{DB: db}
		req := httptest.NewRequest(http.MethodGet, "/pipelines", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		resp := decodeListResponse(t, rec.Body)
		if resp.TotalCount != n {
			t.Errorf("totalCount: want %d, got %d", n, resp.TotalCount)
		}
		if len(resp.Items) != n {
			t.Errorf("items count: want %d, got %d", n, len(resp.Items))
		}
	})
}

func TestListPipelinesFiltering(t *testing.T) {
	db := newTestDB(t)

	// 3 pending for projectA
	const nA = 3
	for range nA {
		seedRecord(t, db, ulid.Make().String(), "projectA", "delivery-a")
	}
	// 2 ok for projectB
	const nBOK = 2
	for range nBOK {
		seedCompletedRecord(t, db, ulid.Make().String(), "projectB", "delivery-b", "out", nil)
	}
	// 2 error for projectC
	const nCErr = 2
	for range nCErr {
		seedCompletedRecord(t, db, ulid.Make().String(), "projectC", "delivery-c", "out", errors.New("fail"))
	}
	// 1 pending for projectB
	seedRecord(t, db, ulid.Make().String(), "projectB", "delivery-b2")

	const total = nA + nBOK + nCErr + 1

	request := func(t *testing.T, query string) listResponse {
		t.Helper()
		handler := admin.ListPipelines{DB: db}
		req := httptest.NewRequest(http.MethodGet, "/pipelines?"+query, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return decodeListResponse(t, rec.Body)
	}

	t.Run("no filter returns all records", func(t *testing.T) {
		resp := request(t, "")
		if resp.TotalCount != total {
			t.Errorf("totalCount: want %d, got %d", total, resp.TotalCount)
		}
	})

	t.Run("project exact match", func(t *testing.T) {
		resp := request(t, "project=projectA")
		if resp.TotalCount != nA {
			t.Errorf("totalCount: want %d, got %d", nA, resp.TotalCount)
		}
		for _, item := range resp.Items {
			if item.Project != "projectA" {
				t.Errorf("unexpected project %q in result", item.Project)
			}
		}
	})

	t.Run("project partial match", func(t *testing.T) {
		resp := request(t, "project=project")
		if resp.TotalCount != total {
			t.Errorf("totalCount: want %d, got %d", total, resp.TotalCount)
		}
	})

	t.Run("deliveryId filter", func(t *testing.T) {
		resp := request(t, "deliveryId=delivery-a")
		if resp.TotalCount != nA {
			t.Errorf("totalCount: want %d, got %d", nA, resp.TotalCount)
		}
	})

	t.Run("status=ok", func(t *testing.T) {
		resp := request(t, "status=ok")
		if resp.TotalCount != nBOK {
			t.Errorf("totalCount: want %d, got %d", nBOK, resp.TotalCount)
		}
	})

	t.Run("status=error", func(t *testing.T) {
		resp := request(t, "status=error")
		if resp.TotalCount != nCErr {
			t.Errorf("totalCount: want %d, got %d", nCErr, resp.TotalCount)
		}
	})

	t.Run("status=pending", func(t *testing.T) {
		const wantPending = nA + 1
		resp := request(t, "status=pending")
		if resp.TotalCount != wantPending {
			t.Errorf("totalCount: want %d, got %d", wantPending, resp.TotalCount)
		}
	})

	t.Run("combined project+status filter", func(t *testing.T) {
		resp := request(t, "project=projectB&status=ok")
		if resp.TotalCount != nBOK {
			t.Errorf("totalCount: want %d, got %d", nBOK, resp.TotalCount)
		}
		for _, item := range resp.Items {
			if item.Project != "projectB" {
				t.Errorf("unexpected project %q in filtered result", item.Project)
			}
		}
	})
}

func TestListPipelinesPagination(t *testing.T) {
	db := newTestDB(t)
	const total = 25
	for range total {
		seedRecord(t, db, ulid.Make().String(), "proj", "del")
	}

	handler := admin.ListPipelines{DB: db}

	doRequest := func(t *testing.T, query string) (listResponse, int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/pipelines?"+query, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return decodeListResponse(t, rec.Body), rec.Code
	}

	t.Run("limit restricts page size", func(t *testing.T) {
		resp, _ := doRequest(t, "limit=5")
		if len(resp.Items) != 5 {
			t.Errorf("items: want 5, got %d", len(resp.Items))
		}
		if resp.TotalCount != total {
			t.Errorf("totalCount: want %d, got %d", total, resp.TotalCount)
		}
	})

	t.Run("nextPage absent when all records fit in one page", func(t *testing.T) {
		resp, _ := doRequest(t, fmt.Sprintf("limit=%d", total))
		if resp.NextPage != nil {
			t.Errorf("nextPage: want nil when all records fit, got %q", *resp.NextPage)
		}
	})

	t.Run("offset pagination yields non-overlapping pages", func(t *testing.T) {
		resp1, _ := doRequest(t, "limit=10")
		resp2, _ := doRequest(t, "limit=10&offset=10")

		page1 := collectPipeIDs(resp1.Items)
		for _, item := range resp2.Items {
			if page1[item.PipeID] {
				t.Errorf("pipeId %q appears in both page 1 and page 2", item.PipeID)
			}
		}
	})

	t.Run("offset pagination doesn't affect total count", func(t *testing.T) {
		resp1, _ := doRequest(t, "limit=10")
		if resp1.TotalCount != total {
			t.Fatalf("initial totalCount doesn't match total number of items, want %d, got %d", total, resp1.TotalCount)
		}
		resp2, _ := doRequest(t, "limit=10&offset=10")
		if resp2.TotalCount != total {
			t.Fatalf("second page totalCount doesn't match total number of items, want %d, got %d", total, resp1.TotalCount)
		}
	})

	t.Run("total page with cursor pagination doesn't drift", func(t *testing.T) {
		resp, _ := doRequest(t, "limit=10")
		if resp.TotalCount != total {
			t.Fatalf("initial total count must match total amount of records, want %d, got %d", total, resp.TotalCount)
		}
		if resp.NextPage == nil {
			t.Fatal("expected nextPage on first page")
		}
		for resp.NextPage != nil {
			nextURL, err := url.Parse(*resp.NextPage)
			if err != nil {
				t.Fatalf("parse nextPage URL: %v", err)
			}
			cursor := nextURL.Query().Get("cursor")
			if cursor == "" {
				t.Fatal("nextPage URL missing cursor param")
			}
			resp, _ = doRequest(t, "limit=10&cursor="+url.QueryEscape(cursor))
			if resp.TotalCount != total {
				t.Fatalf("total count must not drift on next page, want %d, got %d", total, resp.TotalCount)
			}
		}
	})

	t.Run("cursor pagination covers all records without overlap or gaps", func(t *testing.T) {
		seen := make(map[string]bool, total)

		resp, _ := doRequest(t, "limit=10")
		if resp.NextPage == nil {
			t.Fatal("expected nextPage on first page")
		}
		for _, item := range resp.Items {
			seen[item.PipeID] = true
		}

		for resp.NextPage != nil {
			nextURL, err := url.Parse(*resp.NextPage)
			if err != nil {
				t.Fatalf("parse nextPage URL: %v", err)
			}
			cursor := nextURL.Query().Get("cursor")
			if cursor == "" {
				t.Fatal("nextPage URL missing cursor param")
			}
			if nextURL.Query().Has("offset") {
				t.Error("nextPage URL must not contain offset param")
			}

			resp, _ = doRequest(t, "limit=10&cursor="+url.QueryEscape(cursor))
			for _, item := range resp.Items {
				if seen[item.PipeID] {
					t.Errorf("duplicate pipeId %q across cursor pages", item.PipeID)
				}
				seen[item.PipeID] = true
			}
		}

		if len(seen) != total {
			t.Errorf("records seen across all pages: want %d, got %d", total, len(seen))
		}
	})

	t.Run("nextPage URL uses publicURL as base", func(t *testing.T) {
		const publicURL = "https://example.com"
		h := admin.ListPipelines{DB: db, PublicURL: publicURL}
		req := httptest.NewRequest(http.MethodGet, "/pipelines?limit=10", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		resp := decodeListResponse(t, rec.Body)

		if resp.NextPage == nil {
			t.Fatal("expected nextPage")
		}
		if !strings.HasPrefix(*resp.NextPage, publicURL) {
			t.Errorf("nextPage %q should start with publicURL %q", *resp.NextPage, publicURL)
		}
	})

	t.Run("nextPage URL is relative when no publicURL", func(t *testing.T) {
		h := admin.ListPipelines{DB: db}
		req := httptest.NewRequest(http.MethodGet, "/pipelines?limit=10", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		resp := decodeListResponse(t, rec.Body)

		if rec.Code != 200 {
			t.Fatalf("non 200 response code: %d", rec.Code)
		}
		if resp.NextPage == nil {
			t.Fatal("expected nextPage")
		}
		if strings.HasPrefix(*resp.NextPage, "http") {
			t.Errorf("nextPage %q should be a relative URL when no publicURL is set", *resp.NextPage)
		}
	})

	t.Run("invalid cursor returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/pipelines?cursor=notvalid", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if got := rec.Code; got != http.StatusBadRequest {
			t.Fatalf("status: want %d, got %d", http.StatusBadRequest, got)
		}
	})

	t.Run("cursor and offset supplied simultaneously is a 400", func(t *testing.T) {
		resp, _ := doRequest(t, "limit=10")
		nextURL, err := url.Parse(*resp.NextPage)
		if err != nil {
			t.Fatalf("parse nextPage URL: %v", err)
		}
		cursor := nextURL.Query().Get("cursor")
		if cursor == "" {
			t.Fatal("nextPage URL missing cursor param")
		}
		req := httptest.NewRequest(http.MethodGet, "/pipelines?limit=10&offset=2&cursor="+url.QueryEscape(cursor), nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if got := rec.Code; got != http.StatusBadRequest {
			t.Errorf("status: want %d, got %d", http.StatusBadRequest, got)
		}
	})
}

func TestGetPipeline(t *testing.T) {
	db := newTestDB(t)
	pipeID := ulid.Make().String()
	seedCompletedRecord(t, db, pipeID, "myproject", "del-123", "hello output", nil)

	handler := admin.GetPipeline{DB: db}

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

func TestGetPipelineOutput(t *testing.T) {
	db := newTestDB(t)
	pipeID := ulid.Make().String()
	const outputText = "line one\nline two\n"
	seedCompletedRecord(t, db, pipeID, "proj", "del", outputText, nil)

	handler := admin.GetPipelineOutput{DB: db}

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

	t.Run("returns empty body for pending record with no output", func(t *testing.T) {
		pendingID := ulid.Make().String()
		seedRecord(t, db, pendingID, "proj", "del")

		req := httptest.NewRequest(http.MethodGet, "/pipelines/"+pendingID+"/output", nil)
		req.SetPathValue("pipeId", pendingID)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := rec.Code; got != http.StatusOK {
			t.Errorf("status: want %d, got %d", http.StatusOK, got)
		}
		if got := rec.Body.String(); got != "" {
			t.Errorf("body: want empty, got %q", got)
		}
	})
}

func TestPipelinesAuthorization(t *testing.T) {
	db := newTestDB(t)

	const (
		user     = "admin"
		password = "secret"
	)

	auth := middleware.WithBasicAuth(user, password)
	handler := auth(admin.ListPipelines{DB: db})

	doRequest := func(username, pass string) int {
		req := httptest.NewRequest(http.MethodGet, "/pipelines", nil)
		if username != "" || pass != "" {
			req.SetBasicAuth(username, pass)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	t.Run("no credentials returns 401", func(t *testing.T) {
		if got := doRequest("", ""); got != http.StatusUnauthorized {
			t.Errorf("status: want %d, got %d", http.StatusUnauthorized, got)
		}
	})

	t.Run("wrong password returns 401", func(t *testing.T) {
		if got := doRequest(user, "wrongpass"); got != http.StatusUnauthorized {
			t.Errorf("status: want %d, got %d", http.StatusUnauthorized, got)
		}
	})

	t.Run("wrong username returns 401", func(t *testing.T) {
		if got := doRequest("notadmin", password); got != http.StatusUnauthorized {
			t.Errorf("status: want %d, got %d", http.StatusUnauthorized, got)
		}
	})

	t.Run("correct credentials returns 200", func(t *testing.T) {
		if got := doRequest(user, password); got != http.StatusOK {
			t.Errorf("status: want %d, got %d", http.StatusOK, got)
		}
	})

	t.Run("no auth configured allows unauthenticated access", func(t *testing.T) {
		openHandler := middleware.WithBasicAuth("", "")(admin.ListPipelines{DB: db})
		req := httptest.NewRequest(http.MethodGet, "/pipelines", nil)
		rec := httptest.NewRecorder()
		openHandler.ServeHTTP(rec, req)
		if got := rec.Code; got != http.StatusOK {
			t.Errorf("status: want %d, got %d", http.StatusOK, got)
		}
	})
}

package requestmock

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

type RequestMock struct {
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
}

func (dump RequestMock) ToHttpRequest(url string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, url, bytes.NewBuffer([]byte(dump.Body)))
	for key, value := range dump.Headers {
		req.Header.Add(key, value)
	}
	return req
}

func LoadRequestMock(t *testing.T, fileName string) (request RequestMock) {
	t.Helper()
	jsonFile, err := os.Open(fileName)
	if err != nil {
		t.Error(err)
		return
	}
	defer jsonFile.Close()

	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		t.Error(err)
		return
	}

	if err := json.Unmarshal(byteValue, &request); err != nil {
		t.Error(err)
		return
	}

	return request
}

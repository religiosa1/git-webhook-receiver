package handlers_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
)

/*
Go [doesn't preserve](https://github.com/golang/go/issues/27179) the order of
the json fields. This makes the actual payload to change, and those the HMAC
signature values will be different.

For now, I inserted the signature values as they will appear after golang's
alphabetical sorting, but this defeats the purpose of testing on a real git
provider response.

We can switch to storing the response body as a string, but it will be much
less convenient to edit. Preferrably.
*/

type RequestDump struct {
	Secret  string                 `json:"secret"`
	Body    map[string]interface{} `json:"body"`
	Headers map[string]string      `json:"headers"`
}

func (dump RequestDump) ToHttpRequest(url string) (*http.Request, error) {
	body, err := json.Marshal(dump.Body)
	if err != nil {
		return nil, err
	}

	req := httptest.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	for key, value := range dump.Headers {
		req.Header.Add(key, value)
	}
	return req, nil
}

// Load a webhook request dump from a JSON file
func LoadRequestMock(fileName string) (*RequestDump, error) {
	jsonFile, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	var request RequestDump
	if err := json.Unmarshal(byteValue, &request); err != nil {
		return nil, err
	}

	return &request, nil
}

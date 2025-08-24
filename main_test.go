package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func makeRequest(t *testing.T, method, url string, body io.Reader) *http.Response {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	handleRequest(w, req)
	res := w.Result()
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			t.Fatal(err)
		}
	}(res.Body)

	return res
}

func testStatusCode(t *testing.T, statusCode int, expected int) {
	if statusCode != expected {
		t.Errorf(`Expected %d, got %d`, expected, statusCode)
	}
}

func testBody(t *testing.T, body io.ReadCloser, expected string) {
	data, err := io.ReadAll(body)
	if err != nil {
		t.Errorf(`Expected error to be nil got %v`, err)
	}
	trimmedBody := strings.TrimRightFunc(string(data), func(c rune) bool {
		return c == '\r' || c == '\n'
	})
	if trimmedBody != expected {
		t.Errorf(`Expected message to be '%s', got '%s'`, expected, trimmedBody)
	}
}

func TestInvalidMethod(t *testing.T) {
	res := makeRequest(t, http.MethodGet, `/tunnel`, nil)
	testStatusCode(t, res.StatusCode, http.StatusMethodNotAllowed)
	testBody(t, res.Body, `Method not allowed`)
}

func TestBodyMissing(t *testing.T) {
	res := makeRequest(t, http.MethodPost, `/tunnel`, nil)
	testStatusCode(t, res.StatusCode, http.StatusBadRequest)
	testBody(t, res.Body, `Request body is missing`)
}

func TestEmptyBody(t *testing.T) {
	res := makeRequest(t, http.MethodPost, `/tunnel`, bytes.NewBuffer([]byte{}))
	testStatusCode(t, res.StatusCode, http.StatusBadRequest)
	testBody(t, res.Body, `Invalid JSON header: unexpected end of JSON input`)
}

func TestMissingDSN(t *testing.T) {
	json := []byte(`{}`)
	res := makeRequest(t, http.MethodPost, `/tunnel`, bytes.NewBuffer(json))
	testStatusCode(t, res.StatusCode, http.StatusBadRequest)
	testBody(t, res.Body, `Invalid DSN format`)
}

func TestInvalidDSN(t *testing.T) {
	json := []byte(`{"dsn":"https://example.com"}`)
	res := makeRequest(t, http.MethodPost, `/tunnel`, bytes.NewBuffer(json))
	testStatusCode(t, res.StatusCode, http.StatusBadRequest)
	testBody(t, res.Body, `Invalid DSN format`)
}

func TestWrongHost(t *testing.T) {
	t.Setenv(`SENTRY_HOST`, `https://example.com`)

	json := []byte(`{"dsn":"https://example.net/4"}`)
	res := makeRequest(t, http.MethodPost, `/tunnel`, bytes.NewBuffer(json))
	testStatusCode(t, res.StatusCode, http.StatusBadRequest)
	testBody(t, res.Body, `Invalid Sentry hostname: example.net`)
}

func TestWrongProjectId(t *testing.T) {
	t.Setenv(`SENTRY_PROJECT_IDS`, `3,6,8`)

	json := []byte(`{"dsn":"https://example.net/5"}`)
	res := makeRequest(t, http.MethodPost, `/tunnel`, bytes.NewBuffer(json))
	testStatusCode(t, res.StatusCode, http.StatusBadRequest)
	testBody(t, res.Body, `Invalid Sentry project ID: 5`)
}

func TestSuccessfulRequest(t *testing.T) {
	testContent := `LOREMLIPSUM`
	testResponse := `OK`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf(`Expected method to be '%s', got '%s'`, http.MethodPost, r.Method)
		}
		if r.URL.Path != `/api/17/envelope/` {
			t.Errorf(`Expected path to be '%s', got '%s'`, `/api/17/envelope/`, r.URL.Path)
		}
		if _, err := w.Write([]byte(testResponse)); err != nil {
			t.Errorf(`Expected error to be nil got %v`, err)
		}
		contentBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf(`Expected error to be nil got %v`, err)
		}
		content := string(contentBytes)
		if content == `` {
			t.Errorf(`Expected body to be non-empty`)
		}
		contentSplit := strings.Split(content, "\n")
		if len(contentSplit) != 2 {
			t.Errorf(`Expected body to have 2 lines, got %d`, len(contentSplit))
		}
		if contentSplit[1] != testContent {
			t.Errorf(`Expected body to contain '%s', got '%s'`, testContent, contentSplit[1])
		}
	}))
	defer server.Close()

	json := []byte(`{"dsn":"` + server.URL + `/17"}` + "\n" + testContent)
	res := makeRequest(t, http.MethodPost, `/tunnel`, bytes.NewBuffer(json))
	testStatusCode(t, res.StatusCode, http.StatusOK)
	testBody(t, res.Body, testResponse)
}

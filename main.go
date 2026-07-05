package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
)

type SentryHeader struct {
	DSN string `json:"dsn"`
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	var (
		sentryHost       = os.Getenv(`SENTRY_HOST`)
		sentryProjectIDs = os.Getenv(`SENTRY_PROJECT_IDS`)
	)

	slog.Debug(`Handle request`, `url`, r.URL)

	if r.Method != http.MethodPost {
		slog.Debug(`Method not allowed`, `method`, r.Method)
		http.Error(w, `Method not allowed`, http.StatusMethodNotAllowed)
		return
	}

	if r.Body == nil {
		slog.Debug(`Request body is missing`)
		http.Error(w, `Request body is missing`, http.StatusBadRequest)
		return
	}

	contentBytes, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Debug(`Failed to read request body`, `error`, err)
		http.Error(w, `Failed to read request body`, http.StatusInternalServerError)
		return
	}

	content := string(contentBytes)
	firstLine := strings.Split(content, "\n")[0]

	var header SentryHeader
	if err := json.Unmarshal([]byte(firstLine), &header); err != nil {
		errMsg := fmt.Sprintf(`Invalid JSON header: %v`, err)
		slog.Debug(errMsg, `error`, err)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	dsn, err := url.Parse(header.DSN)
	if err != nil || dsn.Host == `` {
		slog.Debug(`Invalid DSN format: failed to parse host`, `dsn`, header.DSN)
		http.Error(w, `Invalid DSN format`, http.StatusBadRequest)
		return
	}

	projectID := strings.Trim(dsn.Path, `/`)
	if projectID == `` {
		slog.Debug(`Invalid DSN format: failed to read project id`, `dsn`, header.DSN)
		http.Error(w, `Invalid DSN format`, http.StatusBadRequest)
		return
	}

	if len(sentryHost) > 0 && dsn.Host != sentryHost {
		errMsg := fmt.Sprintf(`Invalid Sentry hostname: %s`, dsn.Hostname())
		slog.Debug(errMsg, `dsn`, header.DSN, `host`, sentryHost)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	if len(sentryProjectIDs) > 0 && !slices.Contains(strings.Split(sentryProjectIDs, `,`), projectID) {
		errMsg := fmt.Sprintf(`Invalid Sentry project ID: %s`, projectID)
		slog.Debug(errMsg, `dsn`, header.DSN, `projectId`, projectID)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	upstreamSentryURL := fmt.Sprintf(`%s://%s/api/%s/envelope/`, dsn.Scheme, dsn.Host, projectID)

	slog.Debug(`New upstream request`, `upstreamSentryURL`, upstreamSentryURL)

	req, err := http.NewRequest(http.MethodPost, upstreamSentryURL, bytes.NewBuffer(contentBytes))
	if err != nil {
		slog.Debug(`Failed to create upstream request`, `url`, upstreamSentryURL)
		http.Error(w, `Failed to create upstream request`, http.StatusInternalServerError)
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error(`Error tunneling to Sentry`, `error`, err)
		http.Error(w, `Error tunneling to Sentry`, http.StatusInternalServerError)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			slog.Error(`Error closing body`, `error`, err)
		}
	}(resp.Body)

	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		slog.Error(`Error copying response body`, `error`, err)
	}
}

func initLogger(loggingLevel string) {
	var level slog.Level
	err := level.UnmarshalText([]byte(strings.ToUpper(loggingLevel)))
	if err != nil {
		slog.Warn(`Invalid logging level, fallback to 'INFO'`, `error`, err)
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))

	slog.SetDefault(logger)
}

var (
	loggingLevel     = os.Getenv(`LOGGING_LEVEL`)
	path             = os.Getenv(`TUNNEL_PATH`)
	port             = os.Getenv(`PORT`)
	sentryHost       = os.Getenv(`SENTRY_HOST`)
	sentryProjectIDs = os.Getenv(`SENTRY_PROJECT_IDS`)
)

func main() {
	if loggingLevel == `` {
		loggingLevel = `INFO`
	}
	if path == `` {
		path = `/tunnel`
	}
	if port == `` {
		port = `8090`
	}

	initLogger(loggingLevel)

	if sentryHost != `` {
		slog.Info(fmt.Sprintf(`Required Sentry host: %s`, sentryHost), `host`, sentryHost)
	} else {
		slog.Warn(`Allow all Sentry hosts (not recommended, please use 'SENTRY_HOST'-env)`)
	}

	if sentryProjectIDs != `` {
		slog.Info(fmt.Sprintf(`Allowed Sentry projects: %s`, sentryProjectIDs), `projects`, sentryProjectIDs)
	} else {
		slog.Warn(`Allow all project ids (not recommended, please use 'SENTRY_PROJECT_IDS'-env)`)
	}

	http.HandleFunc(path, handleRequest)

	slog.Info(fmt.Sprintf(`Listening on :%s`, port), `port`, port)
	slog.Info(fmt.Sprintf(`Expecting requests to path '%s'`, path), `path`, path)

	err := http.ListenAndServe(`:`+port, nil)
	if err != nil {
		slog.Error(`Failed to listen and serve`, `error`, err)
		os.Exit(1)
		return
	}
}

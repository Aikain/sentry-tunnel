package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

	if r.Method != http.MethodPost {
		http.Error(w, `Method not allowed`, http.StatusMethodNotAllowed)
		return
	}

	if r.Body == nil {
		http.Error(w, `Request body is missing`, http.StatusBadRequest)
		return
	}

	contentBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `Failed to read request body`, http.StatusInternalServerError)
		return
	}

	content := string(contentBytes)
	firstLine := strings.Split(content, "\n")[0]

	var header SentryHeader
	if err := json.Unmarshal([]byte(firstLine), &header); err != nil {
		http.Error(w, fmt.Sprintf(`Invalid JSON header: %v`, err), http.StatusBadRequest)
		return
	}

	dsn, err := url.Parse(header.DSN)
	if err != nil || dsn.Host == `` {
		http.Error(w, `Invalid DSN format`, http.StatusBadRequest)
		return
	}

	projectID := strings.Trim(dsn.Path, `/`)
	if projectID == `` {
		http.Error(w, `Invalid DSN format`, http.StatusBadRequest)
		return
	}

	if len(sentryHost) > 0 && dsn.Host != sentryHost {
		http.Error(w, fmt.Sprintf(`Invalid Sentry hostname: %s`, dsn.Hostname()), http.StatusBadRequest)
		return
	}

	if len(sentryProjectIDs) > 0 && !slices.Contains(strings.Split(sentryProjectIDs, `,`), projectID) {
		http.Error(w, fmt.Sprintf(`Invalid Sentry project ID: %s`, projectID), http.StatusBadRequest)
		return
	}

	upstreamSentryURL := fmt.Sprintf(`%s://%s/api/%s/envelope/`, dsn.Scheme, dsn.Host, projectID)

	req, err := http.NewRequest(http.MethodPost, upstreamSentryURL, bytes.NewBuffer(contentBytes))
	if err != nil {
		http.Error(w, `Failed to create upstream request`, http.StatusInternalServerError)
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf(`Error tunneling to Sentry: %v`, err)
		http.Error(w, `Error tunneling to Sentry`, http.StatusInternalServerError)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf(`Error closing body: %v`, err)
		}
	}(resp.Body)

	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf(`Error copying response body: %v`, err)
	}
}

var (
	path             = os.Getenv(`TUNNEL_PATH`)
	port             = os.Getenv(`PORT`)
	sentryHost       = os.Getenv(`SENTRY_HOST`)
	sentryProjectIDs = os.Getenv(`SENTRY_PROJECT_IDS`)
)

func main() {
	if path == `` {
		path = `/tunnel`
	}
	if port == `` {
		port = `8090`
	}

	if sentryHost != `` {
		log.Printf(`Required Sentry host: %s`, sentryHost)
	} else {
		log.Println(`Allow all Sentry hosts (not recommended, please use 'SENTRY_HOST'-env)`)
	}

	if sentryProjectIDs != `` {
		log.Printf(`Allowed Sentry projects: %s`, sentryProjectIDs)
	} else {
		log.Println(`Allow all project ids (not recommended, please use 'SENTRY_PROJECT_IDS'-env)`)
	}

	http.HandleFunc(path, handleRequest)

	log.Printf(`Listening on :%s`, port)
	log.Printf(`Expecting requests to path '%s'`, path)

	err := http.ListenAndServe(`:`+port, nil)
	if err != nil {
		log.Fatal(err)
		return
	}
}

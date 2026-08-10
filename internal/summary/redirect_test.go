package summary

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientGenerateRejectsRedirect_whenDestinationOriginChanges(t *testing.T) {
	// Given an LLM endpoint that redirects a credential-bearing request to another port.
	destinationHit := make(chan struct{}, 1)
	destination := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		destinationHit <- struct{}{}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"{\"summary\":\"redirected\"}"}}]}`))
	}))
	t.Cleanup(destination.Close)
	source := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer synthetic-key" {
			t.Errorf("source request did not contain its endpoint credential")
		}
		http.Redirect(writer, request, destination.URL+"/v1/chat/completions", http.StatusTemporaryRedirect)
	}))
	t.Cleanup(source.Close)
	client, err := NewClient(Config{
		BaseURL: source.URL + "/v1", Model: "test-model", APIKey: "synthetic-key", Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	// When the client receives the cross-origin redirect.
	_, err = client.Generate(context.Background(), Request{Language: "en", Kind: "daily", Facts: []string{"fact"}})

	// Then it reports the failure without contacting the redirect destination.
	if err == nil {
		t.Fatal("generate returned nil error")
	}
	select {
	case <-destinationHit:
		t.Fatal("redirect destination received the credential-bearing request")
	default:
	}
}

func TestClientGenerateFollowsRedirect_whenDestinationOriginMatches(t *testing.T) {
	// Given an LLM endpoint that redirects within its own origin.
	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/v1/chat/completions" {
			http.Redirect(writer, request, upstream.URL+"/redirected", http.StatusTemporaryRedirect)
			return
		}
		if request.Header.Get("Authorization") != "Bearer synthetic-key" {
			t.Errorf("redirected request did not retain its endpoint credential")
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"{\"summary\":\"same origin\"}"}}]}`))
	}))
	t.Cleanup(upstream.Close)
	client, err := NewClient(Config{
		BaseURL: upstream.URL + "/v1", Model: "test-model", APIKey: "synthetic-key", Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	// When the client receives the same-origin redirect.
	value, err := client.Generate(context.Background(), Request{Language: "en", Kind: "daily", Facts: []string{"fact"}})

	// Then the redirected request succeeds normally.
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if value != "same origin" {
		t.Fatalf("summary = %q, want same origin", value)
	}
}

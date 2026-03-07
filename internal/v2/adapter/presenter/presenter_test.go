package presenter

import (
	"context"
	"testing"
)

func TestPresenterInterface(t *testing.T) {
	var svc Presenter = dummyPresenter{}
	req := PresentRequest{SpecID: "spec-alpha"}
	if _, err := svc.Present(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPresentResponseIncludesPublishedURL(t *testing.T) {
	const publishedURL = "https://example.com/pr/1"
	resp := PresentResponse{Destination: "gh", Message: "ok", PublishedURL: publishedURL}
	if resp.PublishedURL != publishedURL {
		t.Fatalf("PublishedURL mismatch: got %q want %q", resp.PublishedURL, publishedURL)
	}
}

type dummyPresenter struct{}

func (dummyPresenter) Present(_ context.Context, _ PresentRequest) (PresentResponse, error) {
	return PresentResponse{Destination: "none", Message: "ok"}, nil
}

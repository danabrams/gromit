package presenter

import (
    "context"
    "testing"
)

func TestPresenterInterface(t *testing.T) {
    var svc Presenter = (*dummyPresenter)(nil)
    req := PresentRequest{SpecID: "spec-alpha"}
    if _, err := svc.Present(context.Background(), req); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}

type dummyPresenter struct{}

func (dummyPresenter) Present(_ context.Context, _ PresentRequest) (PresentResponse, error) {
    return PresentResponse{Destination: "none", Message: "ok"}, nil
}

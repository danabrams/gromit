package presenter

import (
    "context"
    "testing"
)

func TestPresenterContract(t *testing.T) {
    var _ interface {
        Present(context.Context, PresenterPresentRequest) (PresenterPresentResponse, error)
    } = (Presenter)(nil)
}

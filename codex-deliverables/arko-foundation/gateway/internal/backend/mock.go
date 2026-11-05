package backend

import (
	"context"
	"errors"
)

var ErrEmptyDisplayName = errors.New("display name is required")

type Mock struct{}

func (Mock) Authenticate(_ context.Context, req AuthRequest) (Principal, error) {
	if req.DisplayName == "" {
		return Principal{}, ErrEmptyDisplayName
	}
	return Principal{ID: "mock:demo", BackendMode: "MOCK"}, nil
}

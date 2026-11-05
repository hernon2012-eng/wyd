package backend

import "context"

type AuthRequest struct {
	DisplayName string
}

type Principal struct {
	ID          string `json:"principal_id"`
	BackendMode string `json:"backend_mode"`
}

type Backend interface {
	Authenticate(context.Context, AuthRequest) (Principal, error)
}

package auth

import (
	"context"
	"errors"
	"testing"
)

func TestCreateUserRequiresProjectEvenWithFullAccess(t *testing.T) {
	_, err := (&Service{}).CreateUser(context.Background(), CreateUserInput{Username: "admin", Password: "password123", Permissions: []string{"*"}})
	if !errors.Is(err, ErrProjectsRequired) {
		t.Fatalf("missing project: %v", err)
	}
}

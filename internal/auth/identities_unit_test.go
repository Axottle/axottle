package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// These exercise the parts of CreateUser and the error mapping that never touch
// the database, so they run without a Postgres (the DB-backed behaviours live in
// the *_integration_test.go files gated on a reachable test DB).

func TestCreateUserRequiresALoginMethod(t *testing.T) {
	s := &Service{} // db is never reached: validation fails first
	_, err := s.CreateUser(context.Background(), CreateUserInput{DisplayName: "No Login"})
	if !errors.Is(err, ErrNoLoginMethod) {
		t.Fatalf("want ErrNoLoginMethod, got %v", err)
	}
}

func TestCreateUserPasswordOnlyNeedsUsername(t *testing.T) {
	s := &Service{}
	_, err := s.CreateUser(context.Background(), CreateUserInput{Password: "hunter2hunter2"})
	if !errors.Is(err, ErrEmptyUsername) {
		t.Fatalf("want ErrEmptyUsername, got %v", err)
	}
}

func TestCreateUserRejectsUnknownStatus(t *testing.T) {
	s := &Service{}
	_, err := s.CreateUser(context.Background(), CreateUserInput{
		Password: "hunter2hunter2", Username: "u", Status: "banished", ProjectIDs: []string{"project"},
	})
	if err == nil {
		t.Fatal("want an error for an unknown status")
	}
}

func TestMapCreateUserErr(t *testing.T) {
	cases := []struct {
		name       string
		code       string
		constraint string
		want       error
	}{
		{"dup username", "23505", "admins_username_key", ErrUsernameTaken},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pg := &pgconn.PgError{Code: tc.code, ConstraintName: tc.constraint}
			if got := mapCreateUserErr(pg); !errors.Is(got, tc.want) {
				t.Fatalf("mapCreateUserErr(%s/%s) = %v, want %v", tc.code, tc.constraint, got, tc.want)
			}
		})
	}
	// An unrelated error is wrapped, not mapped to a sentinel.
	other := errors.New("connection reset")
	if got := mapCreateUserErr(other); !errors.Is(got, other) || errors.Is(got, ErrUsernameTaken) {
		t.Fatalf("unrelated error must not map to a create sentinel: %v", got)
	}
}

func TestIsSuperuserTriggerViolation(t *testing.T) {
	hit := &pgconn.PgError{Code: "P0001", Message: "bootstrap superuser cannot be deleted"}
	if !isSuperuserTriggerViolation(hit) {
		t.Fatal("want true for the protect trigger's P0001 message")
	}
	// Same code, unrelated message → not ours.
	miss := &pgconn.PgError{Code: "P0001", Message: "some other raise"}
	if isSuperuserTriggerViolation(miss) {
		t.Fatal("must not match an unrelated P0001")
	}
	if isSuperuserTriggerViolation(errors.New("plain")) {
		t.Fatal("must not match a non-pg error")
	}
}

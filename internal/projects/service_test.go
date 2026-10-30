package projects

import (
	"errors"
	"strings"
	"testing"
)

func TestProjectUpdateValidation(t *testing.T) {
	str := func(s string) *string { return &s }
	for _, tc := range []struct {
		name  string
		req   UpdateRequest
		valid bool
	}{
		{"empty request", UpdateRequest{}, false},
		{"empty name", UpdateRequest{Name: str(" \t\n")}, false},
		{"long name", UpdateRequest{Name: str(strings.Repeat("я", 121))}, false},
		{"unicode boundary", UpdateRequest{Name: str(strings.Repeat("я", 120))}, true},
		{"clear description", UpdateRequest{Description: str("")}, true},
		{"long description", UpdateRequest{Description: str(strings.Repeat("я", 2001))}, false},
		{"trim whitespace", UpdateRequest{Name: str("  Основной  ")}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.req.Validate()
			if tc.valid && err != nil {
				t.Fatal(err)
			}
			if !tc.valid && !errors.Is(err, ErrInvalid) {
				t.Fatalf("expected validation error, got %v", err)
			}
			if tc.valid && got.Name != nil && *got.Name != strings.TrimSpace(*tc.req.Name) {
				t.Fatalf("name was not trimmed: %q", *got.Name)
			}
		})
	}
}

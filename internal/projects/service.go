// Package projects owns project identity and the explicitly scoped inventory.
// HTTP project selection is checked against explicit account membership.
package projects

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/axottle/axottle/internal/auth"
)

var ErrNotFound = errors.New("project not found")
var ErrInvalid = errors.New("invalid project")

type Project struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Connection  *Connection `json:"connection"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// Connection deliberately excludes credentials and internal runtime settings.
type Connection struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	APIURL string `json:"api_url"`
	Status string `json:"status"`
}

type UpdateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

func (r UpdateRequest) Validate() (UpdateRequest, error) {
	if r.Name == nil && r.Description == nil {
		return r, fmt.Errorf("%w: name or description is required", ErrInvalid)
	}
	if r.Name != nil {
		name := strings.TrimSpace(*r.Name)
		if n := utf8.RuneCountInString(name); n < 1 || n > 120 {
			return r, fmt.Errorf("%w: name must contain 1 to 120 characters", ErrInvalid)
		}
		r.Name = &name
	}
	if r.Description != nil {
		description := strings.TrimSpace(*r.Description)
		if utf8.RuneCountInString(description) > 2000 {
			return r, fmt.Errorf("%w: description must contain at most 2000 characters", ErrInvalid)
		}
		r.Description = &description
	}
	return r, nil
}

type Service struct {
	db     *sql.DB
	enrich func(context.Context, []Project) error
}

func NewService(db *sql.DB) *Service { return &Service{db: db} }

// Create adds a project without creating or requiring infrastructure.
func (s *Service) Create(ctx context.Context, name, description string) (*Project, error) {
	req, err := (UpdateRequest{Name: &name, Description: &description}).Validate()
	if err != nil {
		return nil, err
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, err
	}
	return scanProject(s.db.QueryRowContext(ctx, `INSERT INTO projects(id,name,description) VALUES($1,$2,$3)
	 RETURNING id,name,description,created_at,updated_at`, "project_"+hex.EncodeToString(id[:]), *req.Name, *req.Description))
}

const projectSelect = `SELECT p.id, p.name, p.description, p.created_at, p.updated_at FROM projects p`

func scanProject(row interface{ Scan(...any) error }) (*Project, error) {
	var p Project
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Service) enrichProject(ctx context.Context, p *Project, err error) (*Project, error) {
	if err != nil {
		return nil, err
	}
	list := []Project{*p}
	if s.enrich != nil {
		if err := s.enrich(ctx, list); err != nil {
			return nil, err
		}
	}
	return &list[0], nil
}

func (s *Service) List(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, projectSelect+` WHERE `+membershipPredicate(ctx)+` ORDER BY p.created_at, p.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Project{}
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if s.enrich != nil {
		if err := s.enrich(ctx, list); err != nil {
			return nil, err
		}
	}
	return list, nil
}

func (s *Service) Get(ctx context.Context, id string) (*Project, error) {
	p, err := scanProject(s.db.QueryRowContext(ctx, projectSelect+` WHERE p.id = $1 AND `+membershipPredicate(ctx), id))
	return s.enrichProject(ctx, p, err)
}

func (s *Service) Update(ctx context.Context, id string, req UpdateRequest) (*Project, error) {
	req, err := req.Validate()
	if err != nil {
		return nil, err
	}
	// RETURNING keeps the response tied to this write, including concurrent edits.
	p, err := scanProject(s.db.QueryRowContext(ctx, `UPDATE projects p
 SET name=COALESCE($2,name), description=COALESCE($3,description), updated_at=NOW()
 WHERE p.id=$1 AND `+membershipPredicate(ctx)+`
 RETURNING p.id,p.name,p.description,p.created_at,p.updated_at`, id, req.Name, req.Description))
	return s.enrichProject(ctx, p, err)
}

// Background workers have no interactive principal and retain their explicit
// workspace scope. An authenticated non-superuser must hold a current grant.
func membershipPredicate(ctx context.Context) string {
	a := auth.AdminFromContext(ctx)
	if a == nil || a.IsBootstrapSuperuser {
		return "TRUE"
	}
	id := strings.ReplaceAll(a.ID, "'", "''")
	return "(EXISTS (SELECT 1 FROM admins a WHERE a.id='" + id + "' AND a.global_projects) OR EXISTS (SELECT 1 FROM admin_projects ap WHERE ap.project_id=p.id AND ap.admin_id='" + id + "'))"
}

package coreapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Public presentation metadata is intentionally a narrow projection. It never
// exposes arbitrary settings or authentication/integration configuration.
func (s *Server) presentation(w http.ResponseWriter, r *http.Request) {
	values := map[string]string{"name": "Axottle", "language": "ru", "timezone": "UTC"}
	rows, err := s.db.QueryContext(r.Context(), `SELECT key,value_json FROM system_settings WHERE key IN ('panel.name','panel.language','panel.timezone')`)
	if err != nil {
		failure(w, 503, "panel preferences unavailable")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var raw []byte
		if err := rows.Scan(&key, &raw); err != nil {
			failure(w, 503, "panel preferences unavailable")
			return
		}
		var value string
		if json.Unmarshal(raw, &value) != nil {
			continue
		}
		switch key {
		case "panel.name":
			if strings.TrimSpace(value) != "" {
				values["name"] = value
			}
		case "panel.language":
			if value == "ru" || value == "en" {
				values["language"] = value
			}
		case "panel.timezone":
			if _, err := time.LoadLocation(value); err == nil {
				values["timezone"] = value
			}
		}
	}
	if rows.Err() != nil {
		failure(w, 503, "panel preferences unavailable")
		return
	}
	respond(w, 200, values)
}

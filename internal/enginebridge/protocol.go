// Package enginebridge defines the local transport between the open Axottle
// core and its separately built engine. It contains no module implementation.
package enginebridge

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
)

const ProtocolVersion = 1
const IdentityPath = "/_axottle/engine/identity"
const PeerHeader = "X-Axottle-Transport-Peer"

type Identity struct {
	Component string `json:"component"`
	Version   string `json:"version"`
	Protocol  int    `json:"protocol"`
}

func EngineIdentity(version string) Identity {
	return Identity{Component: "axottle-engine", Version: version, Protocol: ProtocolVersion}
}

func (id Identity) Validate(version string) error {
	if id.Component != "axottle-engine" || id.Protocol != ProtocolVersion {
		return fmt.Errorf("engine protocol is incompatible")
	}
	if id.Version == "" || id.Version != version {
		return fmt.Errorf("core and engine must belong to the same release")
	}
	return nil
}

func IdentityHandler(version string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(EngineIdentity(version))
	})
}

// RestorePeer is only installed on the engine's Unix listener. External TCP
// listeners must never trust this header. Restoring the transport peer before
// other middleware preserves IP-based access rules and subscription auditing.
func RestorePeer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer := r.Header.Get(PeerHeader)
		r.Header.Del(PeerHeader)
		if host, _, err := net.SplitHostPort(peer); err == nil && net.ParseIP(host) != nil {
			r.RemoteAddr = peer
		} else {
			http.Error(w, "invalid core transport peer", http.StatusBadRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

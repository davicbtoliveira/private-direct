package app

import "net/http"

func (s *Server) handleICEServers(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ice_servers": s.cfg.ICEConfig()})
}

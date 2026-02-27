package web

import (
	"database/sql"
	"io/fs"
	"net/http"
	"time"
)

// Server is the HTTP handler for the web UI.
type Server struct {
	db   *sql.DB
	mux  *http.ServeMux
	tmpl *templateSet
}

// NewServer creates a Server, loads templates, and registers routes.
func NewServer(db *sql.DB) (*Server, error) {
	tmpl, err := loadTemplates()
	if err != nil {
		return nil, err
	}

	s := &Server{db: db, tmpl: tmpl, mux: http.NewServeMux()}
	s.routes()
	return s, nil
}

func (s *Server) routes() {
	// Pages
	s.mux.HandleFunc("GET /{$}", s.handleIndex)
	s.mux.HandleFunc("GET /stats", s.handleStats)
	s.mux.HandleFunc("GET /habits/new", s.handleAddHabitForm)
	s.mux.HandleFunc("POST /habits", s.handleCreateHabit)
	s.mux.HandleFunc("GET /habits/{id}/archive", s.handleArchiveForm)
	s.mux.HandleFunc("POST /habits/{id}/archive", s.handleArchiveHabit)

	// htmx endpoints
	s.mux.HandleFunc("POST /habits/{id}/entry", s.handleRecordEntry)
	s.mux.HandleFunc("POST /backfill/{habitID}/{date}", s.handleBackfillAnswer)
	s.mux.HandleFunc("POST /backfill/skip/{habitID}/{date}", s.handleBackfillSkip)
	s.mux.HandleFunc("GET /promotion/{id}/confirm", s.handlePromotionConfirm)
	s.mux.HandleFunc("POST /promotion/{id}/confirm", s.handlePromote)
	s.mux.HandleFunc("POST /promotion/skip", s.handleSkipPromotion)
	s.mux.HandleFunc("GET /archived", s.handleArchived)

	// Static files
	staticFS, _ := fs.Sub(content, "static")
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// today returns the current date truncated to midnight UTC.
func today() time.Time {
	t := time.Now().UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

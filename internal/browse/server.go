package browse

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

//go:embed static/*
var staticFS embed.FS

type Summary struct {
	ID        string   `json:"id"`
	CreatedAt int64    `json:"created_at"`
	Date      string   `json:"date"`
	Shout     string   `json:"shout,omitempty"`
	VenueName string   `json:"venue_name,omitempty"`
	City      string   `json:"city,omitempty"`
	State     string   `json:"state,omitempty"`
	Country   string   `json:"country,omitempty"`
	Source    string   `json:"source,omitempty"`
	Category  string   `json:"category,omitempty"`
	People    []string `json:"people,omitempty"`
	Photos    []Photo  `json:"photos,omitempty"`
}

type Photo struct {
	ID       string `json:"id,omitempty"`
	URL      string `json:"url"`
	ThumbURL string `json:"thumb_url,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

type Detail struct {
	Summary Summary        `json:"summary"`
	Raw     map[string]any `json:"raw"`
	Pretty  string         `json:"pretty"`
}

type Query struct {
	Query     string
	StartDate string
	EndDate   string
	HasPhotos bool
	Page      int
	PageSize  int
}

type ListResult struct {
	Items    []Summary `json:"items"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
	Total    int       `json:"total"`
}

type Meta struct {
	Count   int    `json:"count"`
	DataDir string `json:"data_dir"`
	DBPath  string `json:"db_path"`
	MinDate string `json:"min_date,omitempty"`
	MaxDate string `json:"max_date,omitempty"`
}

type source interface {
	ArchiveMeta(context.Context) (*Meta, error)
	QueryCheckins(context.Context, Query) ([]Summary, int, error)
	LoadCheckin(context.Context, string) (*Detail, error)
	RandomCheckin(context.Context, Query) (*Detail, error)
}

type Server struct {
	src source
}

func New(src source) *Server {
	return &Server{src: src}
}

func (s *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", "127.0.0.1:8787")
	if err != nil {
		return fmt.Errorf("listen on 127.0.0.1:8787: %w", err)
	}
	defer listener.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/checkins", s.handleList)
	mux.HandleFunc("/api/checkins/", s.handleDetail)
	mux.HandleFunc("/api/random", s.handleRandom)
	mux.HandleFunc("/api/meta", s.handleMeta)

	staticRoot, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("load static ui: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticRoot)))

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()

	url := "http://127.0.0.1:8787"
	_ = openBrowser(url)
	fmt.Printf("Browse UI: %s\n", url)
	fmt.Println("Press Ctrl+C to stop the server.")

	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) handleMeta(w http.ResponseWriter, _ *http.Request) {
	meta, err := s.src.ArchiveMeta(context.Background())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, meta)
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	page := parseInt(r.URL.Query().Get("page"), 1)
	pageSize := parseInt(r.URL.Query().Get("page_size"), 50)
	items, total, err := s.src.QueryCheckins(context.Background(), Query{
		Query:     q,
		StartDate: r.URL.Query().Get("start_date"),
		EndDate:   r.URL.Query().Get("end_date"),
		HasPhotos: r.URL.Query().Get("has_photos") == "1",
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, ListResult{Items: items, Page: page, PageSize: pageSize, Total: total})
}

func (s *Server) handleDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/checkins/")
	id = strings.TrimSpace(id)
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("missing checkin id"))
		return
	}

	detail, err := s.src.LoadCheckin(context.Background(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, detail)
}

func (s *Server) handleRandom(w http.ResponseWriter, r *http.Request) {
	detail, err := s.src.RandomCheckin(context.Background(), Query{
		Query:     strings.TrimSpace(r.URL.Query().Get("q")),
		StartDate: r.URL.Query().Get("start_date"),
		EndDate:   r.URL.Query().Get("end_date"),
		HasPhotos: r.URL.Query().Get("has_photos") == "1",
		Page:      1,
		PageSize:  1,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, detail)
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)
	writeJSON(w, map[string]string{"error": err.Error()})
}

func parseInt(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func openBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "linux":
		cmd = exec.Command("xdg-open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		return nil
	}
	return cmd.Start()
}

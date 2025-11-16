package server

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"Garage48/internal/camera"

	"github.com/gorilla/mux"
)

type Server struct {
	httpServer *http.Server
	cfg        *Config
	reg        *camera.Registry
}

func New(bind string, cfg *Config, reg *camera.Registry) *Server {
	r := mux.NewRouter()
	s := &Server{
		httpServer: &http.Server{
			Addr:              bind,
			Handler:           r,
			ReadHeaderTimeout: 10 * time.Second,
		},
		cfg: cfg,
		reg: reg,
	}
	r.HandleFunc("/", s.handleIndex).Methods("GET")
	r.HandleFunc("/snapshot/{id}.jpg", s.handleSnapshot).Methods("GET")
	r.HandleFunc("/stream/{id}.mjpg", s.handleMJPEG).Methods("GET")
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	return s
}

func (s *Server) ListenAndServe() error              { return s.httpServer.ListenAndServe() }
func (s *Server) Shutdown(ctx context.Context) error { return s.httpServer.Shutdown(ctx) }

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	tpl, err := template.ParseFiles("web/index.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	data := struct {
		Cameras []CameraConfig
	}{Cameras: s.cfg.Cameras}
	_ = tpl.Execute(w, data)
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	cam := s.reg.Get(id)
	if cam == nil {
		http.NotFound(w, r)
		return
	}
	jpg := cam.LatestJPEG()
	if len(jpg) == 0 {
		http.Error(w, "no frame", 503)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(200)
	_, _ = w.Write(jpg)
}

func (s *Server) handleMJPEG(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	cam := s.reg.Get(id)
	if cam == nil {
		http.NotFound(w, r)
		return
	}

	mw := NewMJPEGWriter(w)
	notify := r.Context().Done()
	flusher, _ := w.(http.Flusher)

	seq := cam.Seq()
	for {
		select {
		case <-notify:
			return
		case <-cam.WaitNext(seq):
			seq = cam.Seq()
			frame := cam.LatestJPEG()
			if len(frame) == 0 {
				continue
			}
			if err := mw.WriteFrame(frame); err != nil {
				log.Printf("mjpeg write error: %v", err)
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}

// MJPEG multipart writer
type MJPEGWriter struct {
	w        http.ResponseWriter
	boundary string
	started  bool
}

func NewMJPEGWriter(w http.ResponseWriter) *MJPEGWriter {
	b := "frame"
	w.Header().Set("Connection", "close")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Content-Type", fmt.Sprintf("multipart/x-mixed-replace; boundary=%s", b))
	return &MJPEGWriter{w: w, boundary: b}
}

func (m *MJPEGWriter) WriteFrame(jpeg []byte) error {
	if !m.started {
		// First boundary without leading CRLF is more widely compatible
		if _, err := fmt.Fprintf(m.w, "--%s\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", m.boundary, len(jpeg)); err != nil {
			return err
		}
		m.started = true
	} else {
		if _, err := fmt.Fprintf(m.w, "\r\n--%s\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", m.boundary, len(jpeg)); err != nil {
			return err
		}
	}
	_, err := m.w.Write(jpeg)
	return err
}

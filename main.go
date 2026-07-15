package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)



type Job struct {
	ID          string
	Website     string
	ClayWebhook string
	ClayToken   string

	mu      sync.Mutex
	Events  []Event
	Done    bool
	Failed  bool
	Profile *Profile
	Targets []Target
	ZipPath string
}

type Event struct {
	Step   string `json:"step"`
	Status string `json:"status"` 
	Detail string `json:"detail,omitempty"`
}

func (j *Job) emit(e Event) {
	j.mu.Lock()
	j.Events = append(j.Events, e)
	j.mu.Unlock()
}

func (j *Job) start(step string)        { j.emit(Event{step, "running", ""}) }
func (j *Job) ok(step, detail string)   { j.emit(Event{step, "done", detail}) }
func (j *Job) skip(step, detail string) { j.emit(Event{step, "skipped", detail}) }
func (j *Job) fail(step, detail string) {
	j.emit(Event{step, "error", detail})
	j.mu.Lock()
	j.Failed = true
	j.mu.Unlock()
}

func (j *Job) finish() {
	j.mu.Lock()
	j.Done = true
	j.mu.Unlock()
}

func (j *Job) setProfile(p Profile)  { j.mu.Lock(); j.Profile = &p; j.mu.Unlock() }
func (j *Job) setTargets(t []Target) { j.mu.Lock(); j.Targets = t; j.mu.Unlock() }
func (j *Job) setZip(p string)       { j.mu.Lock(); j.ZipPath = p; j.mu.Unlock() }

func (j *Job) snapshot() ([]Event, bool, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]Event(nil), j.Events...), j.Done, j.Failed
}

var (
	jobsMu sync.Mutex
	jobs   = map[string]*Job{}
)

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func main() {
	loadDotEnv(".env")
	if os.Getenv("GROQ_API_KEY") == "" {
		log.Fatal("GROQ_API_KEY not set (put it in .env)")
	}

	http.HandleFunc("GET /", handleIndex)
	http.HandleFunc("POST /api/jobs", handleCreate)
	http.HandleFunc("GET /api/jobs/{id}/events", handleEvents)
	http.HandleFunc("GET /api/jobs/{id}/result", handleResult)
	http.HandleFunc("GET /api/jobs/{id}/download", handleDownload)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("outbound.sieve on http:
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(indexHTML))
}

func handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Website     string `json:"website"`
		ClayWebhook string `json:"clay_webhook"`
		ClayToken   string `json:"clay_token"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	req.Website = strings.TrimSpace(req.Website)
	if req.Website == "" || strings.ContainsAny(req.Website, " \t\n") {
		http.Error(w, "website required", http.StatusBadRequest)
		return
	}
	if req.ClayWebhook != "" && !strings.HasPrefix(req.ClayWebhook, "https:
		http.Error(w, "clay webhook must be https", http.StatusBadRequest)
		return
	}
	if req.ClayWebhook == "" {
		req.ClayWebhook = os.Getenv("CLAY_WEBHOOK_URL")
		req.ClayToken = os.Getenv("CLAY_WEBHOOK_AUTH")
	}

	job := &Job{ID: newID(), Website: req.Website, ClayWebhook: req.ClayWebhook, ClayToken: req.ClayToken}
	jobsMu.Lock()
	jobs[job.ID] = job
	jobsMu.Unlock()

	
	
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
		defer cancel()
		runPipeline(ctx, job)
	}()

	writeJSON(w, map[string]string{"id": job.ID})
}

func getJob(r *http.Request) *Job {
	jobsMu.Lock()
	defer jobsMu.Unlock()
	return jobs[r.PathValue("id")]
}






func handleEvents(w http.ResponseWriter, r *http.Request) {
	job := getJob(r)
	if job == nil {
		http.NotFound(w, r)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sent := 0
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		events, done, _ := job.snapshot()
		for ; sent < len(events); sent++ {
			b, _ := json.Marshal(events[sent])
			fmt.Fprintf(w, "data: %s\n\n", b)
		}
		if done {
			fmt.Fprint(w, "event: end\ndata: {}\n\n")
			flusher.Flush()
			return
		}
		flusher.Flush()
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
		}
	}
}

func handleResult(w http.ResponseWriter, r *http.Request) {
	job := getJob(r)
	if job == nil {
		http.NotFound(w, r)
		return
	}
	job.mu.Lock()
	defer job.mu.Unlock()
	writeJSON(w, map[string]any{
		"profile":  job.Profile,
		"targets":  job.Targets,
		"done":     job.Done,
		"failed":   job.Failed,
		"download": job.ZipPath != "",
	})
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	job := getJob(r)
	if job == nil {
		http.NotFound(w, r)
		return
	}
	job.mu.Lock()
	path := job.ZipPath
	job.mu.Unlock()
	if path == "" {
		http.Error(w, "not ready", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="gtm-engine.zip"`)
	http.ServeFile(w, r, path)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}



func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			os.Setenv(k, v)
		}
	}
}

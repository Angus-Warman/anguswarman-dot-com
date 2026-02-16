package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type Comment struct {
	Name    string `json:"name"`
	Body    string `json:"body"`
	Created string `json:"created"`
}

var (
	dataFile = "comments.json"
	mu       sync.Mutex
	tmpl     = template.Must(template.New("comments").Parse(fragmentTemplate))
)

func main() {
	ensureDataFile()

	http.HandleFunc("GET /comments", getComments)
	http.HandleFunc("POST /comments", postComment)

	log.Println("Listening on :5002")
	log.Fatal(http.ListenAndServe(":5002", nil))
}

func getComments(w http.ResponseWriter, r *http.Request) {
	renderCommentSection(w)
}

func postComment(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	body := strings.TrimSpace(r.FormValue("body"))
	honeypot := r.FormValue("website")

	if honeypot == "" &&
		name != "" &&
		body != "" &&
		len(name) <= 80 &&
		len(body) <= 1000 {

		mu.Lock()
		comments := loadComments()
		newComment := Comment{
			Name:    name,
			Body:    body,
			Created: time.Now().Format("2006-01-02 15:04"),
		}
		comments = append(comments, newComment)
		saveComments(comments)
		mu.Unlock()
	}

	renderCommentSection(w)
}

func renderCommentSection(w http.ResponseWriter) {
	mu.Lock()
	comments := loadComments()
	mu.Unlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, comments); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func ensureDataFile() {
	if _, err := os.Stat(dataFile); os.IsNotExist(err) {
		os.WriteFile(dataFile, []byte("[]"), 0644)
	}
}

func loadComments() []Comment {
	data, err := os.ReadFile(dataFile)
	if err != nil {
		return []Comment{}
	}
	var comments []Comment
	json.Unmarshal(data, &comments)
	return comments
}

func saveComments(comments []Comment) {
	data, _ := json.MarshalIndent(comments, "", "  ")
	os.WriteFile(dataFile, data, 0644)
}

const fragmentTemplate = `
<div id="comments-wrapper">

  <div id="comments">
    {{if not .}}
      <p><em>No comments yet.</em></p>
    {{else}}
      {{range .}}
        <div class="comment">
          <div class="meta">
            <strong>{{.Name}}</strong>
            <span class="timestamp">{{.Created}}</span>
          </div>
          <div class="body">{{.Body}}</div>
        </div>
      {{end}}
    {{end}}
  </div>

  <form method="POST" hx-post="/comments" hx-target="#comments-wrapper" hx-swap="outerHTML">
    <input type="text" name="name" placeholder="Name" required maxlength="80">
    <textarea name="body" placeholder="Comment..." required maxlength="1000"></textarea>

    <!-- Honeypot -->
    <input type="text" name="website" style="display:none">

    <button type="submit">Post</button>
  </form>

  <style>
    #comments-wrapper { margin-top: 1rem; }
    .comment { margin-bottom: 1rem; }
    .meta { font-size: 0.85em; opacity: 0.7; }
    .body { margin-top: 0.25rem; white-space: pre-wrap; }
    form { margin-top: 1rem; display: flex; flex-direction: column; gap: 0.5rem; }
    textarea { min-height: 80px; resize: vertical; }
  </style>

</div>
`

package main

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type Comment struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Body    string `json:"body"`
	Created string `json:"created"`
}

var (
	commentsFile = "comments.jsonl"
	mu           sync.RWMutex
	tmpl         = template.Must(template.New("comments").Parse(fragmentTemplate))
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

	if honeypot != "" || name == "" || body == "" || len(name) > 80 || len(body) > 1000 {
		return
	}

	mu.Lock()
	newComment := Comment{
		ID:      newUUID(),
		Name:    name,
		Body:    body,
		Created: time.Now().Format("2006-01-02 15:04"),
	}
	saveComment(newComment)
	mu.Unlock()

	renderCommentSection(w)
}

func renderCommentSection(w http.ResponseWriter) {
	mu.RLock()
	comments, err := loadComments()
	mu.RUnlock()

	if err != nil {
		http.Error(w, "comment load error", http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, comments); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func ensureDataFile() {
	if _, err := os.Stat(commentsFile); os.IsNotExist(err) {
		os.WriteFile(commentsFile, []byte(""), 0644)
	}
}

func loadComments() ([]Comment, error) {
	file, err := os.Open(commentsFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	comments := []Comment{}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		var comment Comment
		if err := json.Unmarshal(line, &comment); err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return comments, nil
}

func saveComment(comment Comment) error {
	data, err := json.Marshal(comment)

	if err != nil {
		return err
	}

	f, err := os.OpenFile(commentsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		return err
	}

	f.Write(data)
	f.Write([]byte{'\n'})

	if err := f.Close(); err != nil {
		return err
	}

	return nil
}

func newUUID() string {
	var b [16]byte

	// Unix time in milliseconds, 48 bits
	ms := uint64(time.Now().UnixMilli())
	binary.BigEndian.PutUint64(b[0:8], ms<<16)

	// 10 bits of randomness
	if _, err := rand.Read(b[6:]); err != nil {
		return ""
	}

	// Set V7
	b[6] = (b[6] & 0x0f) | 0x70

	// Set variant RFC 4122
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4],
		b[4:6],
		b[6:8],
		b[8:10],
		b[10:16],
	)
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

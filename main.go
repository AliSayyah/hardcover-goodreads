package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zalando/go-keyring"
)

const endpoint = "https://api.hardcover.app/v1/graphql"
const keyringService = "hardcover-goodreads"
const keyringUser = "hardcover-token"
const portFallbacks = 20

var version = "dev"

var csvColumns = []string{
	"Book Id",
	"Title",
	"Author",
	"Author l-f",
	"Additional Authors",
	"ISBN",
	"ISBN13",
	"My Rating",
	"Average Rating",
	"Publisher",
	"Binding",
	"Number of Pages",
	"Year Published",
	"Original Publication Year",
	"Date Read",
	"Date Added",
	"Bookshelves",
	"Bookshelves with positions",
	"Exclusive Shelf",
	"My Review",
	"Spoiler",
	"Private Notes",
	"Read Count",
	"Owned Copies",
}

var statusToShelf = map[int]string{
	1: "to-read",
	2: "currently-reading",
	3: "read",
	4: "paused",
	5: "did-not-finish",
	6: "ignored",
}

type app struct {
	client  *http.Client
	mu      sync.Mutex
	exports map[string]exportFiles
}

type exportFiles struct {
	csv  []byte
	json []byte
}

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type graphQLResponse[T any] struct {
	Data   T              `json:"data"`
	Errors []graphQLError `json:"errors"`
}

type graphQLError struct {
	Message string `json:"message"`
}

type libraryData struct {
	UserBooks []userBook `json:"user_books"`
}

type userBook struct {
	ID                      int      `json:"id"`
	DateAdded               string   `json:"date_added"`
	FirstReadDate           string   `json:"first_read_date"`
	FirstStartedReadingDate string   `json:"first_started_reading_date"`
	LastReadDate            string   `json:"last_read_date"`
	OwnedCopies             any      `json:"owned_copies"`
	PrivateNotes            string   `json:"private_notes"`
	Rating                  any      `json:"rating"`
	ReadCount               int      `json:"read_count"`
	Review                  string   `json:"review"`
	ReviewHasSpoilers       bool     `json:"review_has_spoilers"`
	ReviewRaw               string   `json:"review_raw"`
	StatusID                int      `json:"status_id"`
	Book                    book     `json:"book"`
	Edition                 *edition `json:"edition"`
}

type book struct {
	ID                 int    `json:"id"`
	CachedContributors any    `json:"cached_contributors"`
	Pages              any    `json:"pages"`
	ReleaseDate        string `json:"release_date"`
	Subtitle           string `json:"subtitle"`
	Title              string `json:"title"`
}

type edition struct {
	CachedContributors any        `json:"cached_contributors"`
	ISBN10             string     `json:"isbn_10"`
	ISBN13             string     `json:"isbn_13"`
	Pages              any        `json:"pages"`
	PhysicalFormat     string     `json:"physical_format"`
	Publisher          *publisher `json:"publisher"`
	ReleaseDate        string     `json:"release_date"`
	Subtitle           string     `json:"subtitle"`
	Title              string     `json:"title"`
}

type publisher struct {
	Name string `json:"name"`
}

type pageData struct {
	Error         string
	Notice        string
	UserID        string
	ExportID      string
	Count         int
	HasSavedToken bool
}

var page = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Hardcover to Goodreads</title>
  <style>
    :root { color-scheme: light dark; font-family: Inter, ui-sans-serif, system-ui, sans-serif; }
    body { margin: 0; background: Canvas; color: CanvasText; }
    main { max-width: 760px; margin: 0 auto; padding: 40px 20px; }
    h1 { font-size: 2rem; margin: 0 0 28px; letter-spacing: 0; }
    form { display: grid; gap: 18px; }
    label { display: grid; gap: 6px; font-weight: 650; }
    label.check { display: flex; gap: 8px; align-items: center; font-weight: 500; }
    input { width: 100%; box-sizing: border-box; font: inherit; padding: 11px 12px; border: 1px solid color-mix(in srgb, CanvasText 25%, transparent); border-radius: 6px; background: Canvas; color: CanvasText; }
    label.check input { width: auto; }
    button, a.button { width: fit-content; font: inherit; font-weight: 700; padding: 11px 14px; border: 0; border-radius: 6px; background: #14532d; color: white; text-decoration: none; cursor: pointer; }
    button.secondary { background: #475569; }
    a.button.secondary { background: #475569; }
    .actions { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; margin-top: 4px; }
    .error { padding: 12px; border-left: 4px solid #b91c1c; background: color-mix(in srgb, #b91c1c 12%, Canvas); }
    .notice { padding: 12px; border-left: 4px solid #14532d; background: color-mix(in srgb, #14532d 12%, Canvas); }
    .result { margin-top: 28px; padding-top: 24px; border-top: 1px solid color-mix(in srgb, CanvasText 16%, transparent); }
    .muted { color: color-mix(in srgb, CanvasText 68%, Canvas); font-size: .95rem; }
  </style>
</head>
<body>
<main>
  <h1>Hardcover to Goodreads</h1>
  {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
  {{if .Notice}}<p class="notice">{{.Notice}}</p>{{end}}
  {{if .HasSavedToken}}<p class="muted">A saved Hardcover token is available. Leave the token blank to use it.</p>{{end}}
  <form method="post" action="/export">
    <label>Hardcover token
      <input name="token" type="password" autocomplete="off" {{if not .HasSavedToken}}required{{end}}>
      <span class="muted">Your Hardcover API token, not Goodreads. Get it from your Hardcover account API page.</span>
    </label>
    <label>Hardcover user ID
      <input name="user_id" inputmode="numeric" autocomplete="off" value="{{.UserID}}" placeholder="optional">
      <span class="muted">Optional. Leave blank and the app will read your Hardcover user ID from the token.</span>
    </label>
    <label class="check"><input name="save_token" type="checkbox"> Save token in OS keychain</label>
    <span class="muted">Saved only after a successful export. You can forget it later from this page.</span>
    <div class="actions">
      <button type="submit">Export</button>
      <a class="button secondary" href="https://hardcover.app/account/api" target="_blank" rel="noreferrer">Open Hardcover API Key Page</a>
      <a href="https://www.goodreads.com/review/import" target="_blank" rel="noreferrer">Goodreads Import</a>
    </div>
  </form>
  {{if .HasSavedToken}}
  <form method="post" action="/forget">
    <button class="secondary" type="submit">Forget saved token</button>
  </form>
  {{end}}
  {{if .ExportID}}
  <section class="result">
    <p>{{.Count}} books exported.</p>
    <div class="actions">
      <a class="button" href="/download/{{.ExportID}}/goodreads_import.csv">CSV</a>
      <a class="button" href="/download/{{.ExportID}}/hardcover_library.json">JSON</a>
    </div>
  </section>
  {{end}}
</main>
</body>
</html>`))

func main() {
	addr := flag.String("addr", ":8080", "address to listen on; if busy, tries the next ports")
	noOpen := flag.Bool("no-open", false, "do not open the browser")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println("hardcover-goodreads", version)
		return
	}

	a := &app{
		client:  &http.Client{Timeout: 30 * time.Second},
		exports: map[string]exportFiles{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", a.index)
	mux.HandleFunc("/export", a.export)
	mux.HandleFunc("/forget", a.forget)
	mux.HandleFunc("/download/", a.download)

	listener, err := listenWithFallback(*addr)
	if err != nil {
		log.Fatal(err)
	}
	url := localURL(listener.Addr().String())
	log.Printf("listening on %s", url)
	if !*noOpen {
		go func() {
			time.Sleep(100 * time.Millisecond)
			if err := openBrowser(url); err != nil {
				log.Printf("could not open browser: %v", err)
			}
		}()
	}
	if err := http.Serve(listener, mux); err != nil {
		log.Fatal(err)
	}
}

func listenWithFallback(addr string) (net.Listener, error) {
	listener, err := net.Listen("tcp", addr)
	if err == nil {
		return listener, nil
	}

	host, portText, splitErr := net.SplitHostPort(addr)
	port, atoiErr := strconv.Atoi(portText)
	if splitErr != nil || atoiErr != nil || port == 0 {
		return nil, err
	}

	lastErr := err
	for next := port + 1; next <= port+portFallbacks && next <= 65535; next++ {
		listener, lastErr = net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(next)))
		if lastErr == nil {
			log.Printf("%s unavailable, using %s", addr, listener.Addr().String())
			return listener, nil
		}
	}
	return nil, fmt.Errorf("could not listen on %s or the next %d ports: %w", addr, portFallbacks, lastErr)
}

func localURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr
	}
	if host == "" || host == "::" || host == "0.0.0.0" || host == "[::]" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func (a *app) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	render(w, newPageData(""))
}

func (a *app) export(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		render(w, pageData{Error: err.Error()})
		return
	}

	rawToken := r.FormValue("token")
	token, err := tokenForExport(rawToken)
	if err != nil {
		render(w, newPageData(err.Error()))
		return
	}

	userID, err := parseUserID(r.Context(), a.client, token, r.FormValue("user_id"))
	if err != nil {
		data := newPageData(err.Error())
		data.UserID = r.FormValue("user_id")
		render(w, data)
		return
	}

	books, err := fetchLibrary(r.Context(), a.client, token, userID)
	if err != nil {
		data := newPageData(err.Error())
		data.UserID = strconv.Itoa(userID)
		render(w, data)
		return
	}

	csvBytes, err := csvFile(books)
	if err != nil {
		data := newPageData(err.Error())
		data.UserID = strconv.Itoa(userID)
		render(w, data)
		return
	}
	jsonBytes, err := json.MarshalIndent(books, "", "  ")
	if err != nil {
		data := newPageData(err.Error())
		data.UserID = strconv.Itoa(userID)
		render(w, data)
		return
	}

	id, err := randomID()
	if err != nil {
		data := newPageData(err.Error())
		data.UserID = strconv.Itoa(userID)
		render(w, data)
		return
	}

	a.mu.Lock()
	// ponytail: local single-user app; add expiry if this becomes a daemon.
	a.exports[id] = exportFiles{csv: csvBytes, json: jsonBytes}
	a.mu.Unlock()

	data := newPageData("")
	data.ExportID = id
	data.Count = len(books)
	data.UserID = strconv.Itoa(userID)
	if r.FormValue("save_token") == "on" && strings.TrimSpace(rawToken) != "" {
		if err := keyring.Set(keyringService, keyringUser, token); err != nil {
			data.Error = "Exported, but could not save token: " + err.Error()
		} else {
			data.Notice = "Token saved in your OS keychain."
			data.HasSavedToken = true
		}
	}
	render(w, data)
}

func (a *app) forget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	err := keyring.Delete(keyringService, keyringUser)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		render(w, newPageData("Could not forget token: "+err.Error()))
		return
	}
	data := newPageData("")
	data.Notice = "Saved token forgotten."
	render(w, data)
}

func (a *app) download(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/download/"), "/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}

	a.mu.Lock()
	files, ok := a.exports[parts[0]]
	a.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}

	switch parts[1] {
	case "goodreads_import.csv":
		w.Header().Set("content-type", "text/csv; charset=utf-8")
		w.Header().Set("content-disposition", `attachment; filename="goodreads_import.csv"`)
		_, _ = w.Write(files.csv)
	case "hardcover_library.json":
		w.Header().Set("content-type", "application/json; charset=utf-8")
		w.Header().Set("content-disposition", `attachment; filename="hardcover_library.json"`)
		_, _ = w.Write(files.json)
	default:
		http.NotFound(w, r)
	}
}

func render(w http.ResponseWriter, data pageData) {
	w.Header().Set("content-type", "text/html; charset=utf-8")
	if err := page.Execute(w, data); err != nil {
		log.Println(err)
	}
}

func newPageData(message string) pageData {
	data := pageData{Error: message}
	if _, err := keyring.Get(keyringService, keyringUser); err == nil {
		data.HasSavedToken = true
	}
	return data
}

func tokenForExport(raw string) (string, error) {
	if token := normalizeToken(raw); token != "" {
		return token, nil
	}
	token, err := keyring.Get(keyringService, keyringUser)
	if err == nil {
		return token, nil
	}
	if errors.Is(err, keyring.ErrNotFound) {
		return "", errors.New("Hardcover token is required.")
	}
	return "", fmt.Errorf("could not read saved token: %w", err)
}

func parseUserID(ctx context.Context, client *http.Client, token, raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw != "" {
		id, err := strconv.Atoi(raw)
		if err != nil || id <= 0 {
			return 0, errors.New("User ID must be a positive number.")
		}
		return id, nil
	}
	return getUserID(ctx, client, token)
}

func getUserID(ctx context.Context, client *http.Client, token string) (int, error) {
	var out struct {
		Me json.RawMessage `json:"me"`
	}
	if err := graphql(ctx, client, token, "query { me { id } }", nil, &out); err != nil {
		return 0, err
	}

	var one struct {
		ID int `json:"id"`
	}
	if json.Unmarshal(out.Me, &one) == nil && one.ID > 0 {
		return one.ID, nil
	}

	var many []struct {
		ID int `json:"id"`
	}
	if json.Unmarshal(out.Me, &many) == nil && len(many) > 0 && many[0].ID > 0 {
		return many[0].ID, nil
	}
	return 0, errors.New("could not read user id from Hardcover response")
}

func fetchLibrary(ctx context.Context, client *http.Client, token string, userID int) ([]userBook, error) {
	const query = `
query Library($userId: Int!, $limit: Int!, $offset: Int!) {
  user_books(
    where: {user_id: {_eq: $userId}},
    order_by: {id: asc},
    limit: $limit,
    offset: $offset
  ) {
    id
    date_added
    first_read_date
    first_started_reading_date
    last_read_date
    owned_copies
    private_notes
    rating
    read_count
    review
    review_has_spoilers
    review_raw
    status_id
    book {
      id
      cached_contributors
      pages
      release_date
      subtitle
      title
    }
    edition {
      cached_contributors
      isbn_10
      isbn_13
      pages
      physical_format
      release_date
      subtitle
      title
      publisher {
        name
      }
    }
  }
}`
	var books []userBook
	for offset := 0; ; offset += 100 {
		var out libraryData
		err := graphql(ctx, client, token, query, map[string]any{
			"userId": userID,
			"limit":  100,
			"offset": offset,
		}, &out)
		if err != nil {
			return nil, err
		}
		books = append(books, out.UserBooks...)
		if len(out.UserBooks) < 100 {
			return books, nil
		}
	}
}

func graphql(ctx context.Context, client *http.Client, token, query string, variables map[string]any, out any) error {
	body, err := json.Marshal(graphQLRequest{Query: query, Variables: variables})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("authorization", token)
	req.Header.Set("content-type", "application/json")
	req.Header.Set("user-agent", "hardcover-goodreads/0.2")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Hardcover API returned HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var payload graphQLResponse[json.RawMessage]
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return err
	}
	if len(payload.Errors) > 0 {
		return errors.New(payload.Errors[0].Message)
	}

	decoder = json.NewDecoder(bytes.NewReader(payload.Data))
	decoder.UseNumber()
	return decoder.Decode(out)
}

func csvFile(books []userBook) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write(csvColumns); err != nil {
		return nil, err
	}
	for _, book := range books {
		if err := writer.Write(csvRow(book)); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return buf.Bytes(), writer.Error()
}

func csvRow(ub userBook) []string {
	ed := ub.Edition
	names := contributorNames(ub.Book.CachedContributors)
	if len(names) == 0 && ed != nil {
		names = contributorNames(ed.CachedContributors)
	}

	shelf := statusToShelf[ub.StatusID]
	if shelf == "" {
		shelf = "to-read"
	}
	readCount := ub.ReadCount
	if readCount == 0 && shelf == "read" {
		readCount = 1
	}

	editionTitle, isbn10, isbn13, binding, publisherName, editionDate, editionPages := "", "", "", "", "", "", any(nil)
	if ed != nil {
		editionTitle = ed.Title
		isbn10 = ed.ISBN10
		isbn13 = ed.ISBN13
		binding = ed.PhysicalFormat
		editionDate = ed.ReleaseDate
		editionPages = ed.Pages
		if ed.Publisher != nil {
			publisherName = ed.Publisher.Name
		}
	}

	releaseDate := editionDate
	if releaseDate == "" {
		releaseDate = ub.Book.ReleaseDate
	}

	return []string{
		"",
		bookTitle(ub.Book.Title, editionTitle),
		first(names),
		"",
		strings.Join(rest(names), ", "),
		goodreadsISBN(isbn10),
		goodreadsISBN(isbn13),
		goodreadsRating(ub.Rating),
		"",
		publisherName,
		binding,
		stringValue(firstNonNil(editionPages, ub.Book.Pages)),
		year(releaseDate),
		year(ub.Book.ReleaseDate),
		goodreadsDate(firstNonEmpty(ub.LastReadDate, ub.FirstReadDate)),
		goodreadsDate(firstNonEmpty(ub.DateAdded, time.Now().Format(time.DateOnly))),
		shelf,
		shelf,
		shelf,
		firstNonEmpty(ub.ReviewRaw, ub.Review),
		boolString(ub.ReviewHasSpoilers),
		ub.PrivateNotes,
		strconv.Itoa(readCount),
		stringValue(ub.OwnedCopies),
	}
}

func contributorNames(value any) []string {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		var decoded any
		if json.Unmarshal([]byte(v), &decoded) == nil {
			return contributorNames(decoded)
		}
		return []string{strings.TrimSpace(v)}
	case []any:
		var names []string
		for _, item := range v {
			names = append(names, contributorNames(item)...)
		}
		return unique(names)
	case map[string]any:
		for _, key := range []string{"name", "author_name"} {
			if s, ok := v[key].(string); ok && strings.TrimSpace(s) != "" {
				return []string{s}
			}
		}
		for _, key := range []string{"author", "authors", "contributor", "contributors", "contributions"} {
			if names := contributorNames(v[key]); len(names) > 0 {
				return names
			}
		}
	}
	return nil
}

func unique(values []string) []string {
	seen := map[string]bool{}
	out := values[:0]
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value != "" && !seen[key] {
			seen[key] = true
			out = append(out, value)
		}
	}
	return out
}

func bookTitle(bookTitle, editionTitle string) string {
	return firstNonEmpty(bookTitle, editionTitle)
}

func goodreadsISBN(value string) string {
	isbn := normalizedISBN(value)
	if isbn == "" {
		return ""
	}
	return `="` + isbn + `"`
}

func normalizedISBN(value string) string {
	var chars []rune
	for _, r := range strings.ToUpper(value) {
		if r >= '0' && r <= '9' {
			chars = append(chars, r)
		} else if r == 'X' {
			chars = append(chars, r)
		}
	}
	isbn := string(chars)
	if len(isbn) == 9 && validISBN10("0"+isbn) {
		return "0" + isbn
	}
	if len(isbn) == 10 && validISBN10(isbn) {
		return isbn
	}
	if len(isbn) == 13 && validISBN13(isbn) {
		return isbn
	}
	return ""
}

func validISBN10(isbn string) bool {
	if len(isbn) != 10 {
		return false
	}
	sum := 0
	for i, r := range isbn {
		var n int
		if r == 'X' && i == 9 {
			n = 10
		} else if r >= '0' && r <= '9' {
			n = int(r - '0')
		} else {
			return false
		}
		sum += n * (10 - i)
	}
	return sum%11 == 0
}

func validISBN13(isbn string) bool {
	if len(isbn) != 13 || !strings.HasPrefix(isbn, "978") && !strings.HasPrefix(isbn, "979") {
		return false
	}
	sum := 0
	for i, r := range isbn {
		if r < '0' || r > '9' {
			return false
		}
		n := int(r - '0')
		if i%2 == 1 {
			n *= 3
		}
		sum += n
	}
	return sum%10 == 0
}

func goodreadsRating(value any) string {
	n, ok := number(value)
	if !ok {
		return "0"
	}
	return strconv.Itoa(max(0, min(5, int(math.Floor(n+0.5)))))
}

func number(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	case string:
		n, err := strconv.ParseFloat(v, 64)
		return n, err == nil
	case int:
		return float64(v), true
	}
	return 0, false
}

func normalizeToken(token string) string {
	token = strings.TrimSpace(token)
	if token != "" && !strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return "Bearer " + token
	}
	return token
}

func goodreadsDate(value string) string { return strings.ReplaceAll(value, "-", "/") }

func year(value string) string {
	if len(value) >= 4 {
		return value[:4]
	}
	return ""
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return ""
}

func stringValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case float64:
		return strconv.Itoa(int(v))
	case json.Number:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func rest(values []string) []string {
	if len(values) < 2 {
		return nil
	}
	return values[1:]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func randomID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

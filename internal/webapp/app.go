package webapp

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"sync"

	"dslparser/Diagnostics"
	"dslparser/internal/pipeline"
	"dslparser/internal/workspace"
	webassets "dslparser/web"
)

// pageData задаёт данные общего HTML-шаблона.
type pageData struct {
	Title  string
	Body   template.HTML
	Status string
}

// Progress является компактным immutable snapshot операции.
type Progress struct {
	Total               int    `json:"total"`
	Processed           int    `json:"processed"`
	Completed           int    `json:"completed"`
	CompletedWithErrors int    `json:"completed_with_errors"`
	Incomplete          int    `json:"incomplete"`
	Excluded            int    `json:"excluded"`
	Remaining           int    `json:"remaining"`
	Current             string `json:"current"`
	Event               string `json:"event"`
	Finished            bool   `json:"finished"`
	Cancelled           bool   `json:"cancelled"`
}

// App владеет всем изменяемым HTTP-состоянием.
type App struct {
	mu        sync.RWMutex
	templates *template.Template
	scanner   workspace.Scanner
	runner    pipeline.Runner
	scan      workspace.Scan
	mode      string
	selected  *string
	progress  Progress
	running   bool
	cancel    bool
	shutdown  func() error
}

// New разбирает assets и создаёт приложение без package-level state.
func New(scanner workspace.Scanner, runner pipeline.Runner, shutdown func() error) (*App, error) {
	data, err := webassets.Files.ReadFile("templates/base.html")
	if err != nil {
		return nil, fmt.Errorf("ASSET_INIT_FAILURE: %w", err)
	}
	templates, err := template.New("base").Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("ASSET_INIT_FAILURE: %w", err)
	}
	return &App{templates: templates, scanner: scanner, runner: runner, mode: "auto", shutdown: shutdown}, nil
}

// Handler возвращает middleware и маршруты приложения.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.home)
	mux.HandleFunc("/workspace", a.workspace)
	mux.HandleFunc("/files", a.files)
	mux.HandleFunc("/analysis/file", a.analysisFile)
	mux.HandleFunc("/analysis/all", a.analysisAll)
	mux.HandleFunc("/progress", a.progressPage)
	mux.HandleFunc("/api/progress", a.progressAPI)
	mux.HandleFunc("/analysis/cancel", a.cancelAnalysis)
	mux.HandleFunc("/report/", a.reportPage)
	mux.HandleFunc("/shutdown", a.shutdownHandler)
	assets, _ := fs.Sub(webassets.Files, "assets")
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets))))
	return a.middleware(mux)
}

// middleware ограничивает Host, заголовки и восстанавливает handler panic.
func (a *App) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'")
		if r.Host != "127.0.0.1:8580" && r.Host != "localhost:8580" {
			http.Error(w, "INVALID_REQUEST: недопустимый Host", http.StatusBadRequest)
			return
		}
		defer func() {
			if recover() != nil {
				http.Error(w, "HTTP_HANDLER_PANIC", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// method проверяет точный HTTP-метод.
func method(w http.ResponseWriter, r *http.Request, want string) bool {
	if r.Method != want {
		w.Header().Set("Allow", want)
		http.Error(w, "INVALID_REQUEST: метод не разрешён", http.StatusMethodNotAllowed)
		return false
	}
	return true
}
func (a *App) render(w http.ResponseWriter, title string, body template.HTML, status string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.templates.ExecuteTemplate(w, "page", pageData{title, body, status}); err != nil {
		http.Error(w, "TEMPLATE_RENDER_FAILURE", 500)
	}
}
func (a *App) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" || !method(w, r, "GET") {
		return
	}
	versions := diagnostics.SupportedVersions()
	body := `<form method="post" action="/workspace"><label for="path">Абсолютный путь</label><input id="path" name="path" required aria-describedby="path-help"><div id="path-help">Каталог со сценариями</div><label for="version">Версия</label><select id="version" name="version"><option value="auto">Автоопределение</option>`
	for _, v := range versions {
		body += `<option value="` + template.HTMLEscapeString(string(v)) + `">` + template.HTMLEscapeString(string(v)) + `</option>`
	}
	body += `</select><button>Продолжить</button></form><form method="post" action="/shutdown"><button>Завершить работу</button></form>`
	a.render(w, "Проверка сценариев", template.HTML(body), "Готово")
}
func (a *App) workspace(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "INVALID_REQUEST", 400)
		return
	}
	scan, err := a.scanner.Scan(r.FormValue("path"))
	if err != nil {
		a.render(w, "Ошибка каталога", template.HTML(`<p class="error">`+template.HTMLEscapeString(err.Error())+`</p><a href="/">Назад</a>`), err.Error())
		return
	}
	mode := "auto"
	var selected *string
	if value := r.FormValue("version"); value != "auto" {
		mode = "explicit"
		v := value
		selected = &v
	}
	a.mu.Lock()
	a.scan = scan
	a.mode = mode
	a.selected = selected
	a.mu.Unlock()
	http.Redirect(w, r, "/files", http.StatusSeeOther)
}
func (a *App) files(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	a.mu.RLock()
	scan := a.scan
	a.mu.RUnlock()
	var b strings.Builder
	b.WriteString(`<form method="post" action="/analysis/all"><button>Проанализировать все</button></form><div class="scroll" role="region" aria-label="Сценарии"><table><caption>Найденные сценарии</caption><thead><tr><th scope="col">Файл</th><th scope="col">Размер</th><th scope="col">Версия DSL</th><th scope="col">Действие</th></tr></thead><tbody>`)
	for _, f := range scan.Files {
		fmt.Fprintf(&b, `<tr><td>%s</td><td>%d</td><td>%s</td><td><form method="post" action="/analysis/file"><input type="hidden" name="id" value="%s"><button>Проанализировать</button></form></td></tr>`, template.HTMLEscapeString(f.Path), f.Size, template.HTMLEscapeString(f.Version), template.HTMLEscapeString(f.ID))
	}
	b.WriteString(`</tbody></table></div>`)
	a.render(w, "Сценарии", template.HTML(b.String()), fmt.Sprintf("Найдено: %d", len(scan.Files)))
}
func (a *App) start(w http.ResponseWriter, r *http.Request, indexes []int) {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		http.Error(w, "OPERATION_IN_PROGRESS", 409)
		return
	}
	a.running = true
	a.cancel = false
	a.progress = Progress{Total: len(indexes), Remaining: len(indexes), Event: "Операция начата"}
	a.mu.Unlock()
	go a.run(indexes)
	http.Redirect(w, r, "/progress", http.StatusSeeOther)
}
func (a *App) run(indexes []int) {
	for _, index := range indexes {
		a.mu.Lock()
		if a.cancel {
			a.progress.Cancelled = true
			a.progress.Event = "Операция отменена"
			a.mu.Unlock()
			break
		}
		file := a.scan.Files[index]
		a.progress.Current = file.Name
		a.progress.Event = "Обрабатывается " + file.Name
		mode, selected := a.mode, a.selected
		a.mu.Unlock()
		result, err := a.runner.Run(file.Path, mode, selected)
		a.mu.Lock()
		a.progress.Processed++
		a.progress.Remaining--
		if err != nil {
			if strings.Contains(err.Error(), "исключена выбором") {
				a.progress.Excluded++
			} else {
				a.progress.Incomplete++
			}
			a.progress.Event = err.Error()
		} else {
			switch result.Analysis.Status {
			case "completed":
				a.progress.Completed++
			case "completed_with_errors":
				a.progress.CompletedWithErrors++
			default:
				a.progress.Incomplete++
			}
		}
		a.mu.Unlock()
	}
	a.mu.Lock()
	a.progress.Finished = true
	a.running = false
	if !a.progress.Cancelled {
		a.progress.Event = "Операция завершена"
	}
	a.mu.Unlock()
}
func (a *App) analysisAll(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	a.mu.RLock()
	n := len(a.scan.Files)
	a.mu.RUnlock()
	indexes := make([]int, n)
	for i := range indexes {
		indexes[i] = i
	}
	a.start(w, r, indexes)
}
func (a *App) analysisFile(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	r.ParseForm()
	index := -1
	a.mu.RLock()
	for i, file := range a.scan.Files {
		if file.ID == r.FormValue("id") {
			index = i
			break
		}
	}
	a.mu.RUnlock()
	if index < 0 {
		http.Error(w, "INVALID_REQUEST", 400)
		return
	}
	a.start(w, r, []int{index})
}
func (a *App) progressPage(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	a.render(w, "Ход анализа", template.HTML(`<section class="card"><p data-progress>Подготовка…</p><form method="post" action="/analysis/cancel"><button>Отменить после текущего файла</button></form><a href="/files">Вернуться к списку</a></section>`), "Ожидание обновления")
}
func (a *App) progressAPI(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	a.mu.RLock()
	p := a.progress
	a.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}
func (a *App) cancelAnalysis(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	a.mu.Lock()
	a.cancel = true
	a.mu.Unlock()
	http.Redirect(w, r, "/progress", 303)
}
func (a *App) reportPage(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	a.render(w, "Отчёт", template.HTML(`<p>Отчёт доступен рядом со сценарием в формате JSON.</p><a href="/files">К списку</a>`), "Отчёт открыт")
}
func (a *App) shutdownHandler(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	a.render(w, "Завершение", template.HTML(`<p>Сервер завершает работу.</p>`), "Завершение работы")
	if a.shutdown != nil {
		go a.shutdown()
	}
}

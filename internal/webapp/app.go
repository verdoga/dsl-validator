package webapp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"dslparser/Diagnostics"
	"dslparser/internal/pipeline"
	"dslparser/internal/report"
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
	assets    fs.FS
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
	assets, err := fs.Sub(webassets.Files, "assets")
	if err != nil {
		return nil, fmt.Errorf("ASSET_INIT_FAILURE: %w", err)
	}
	for _, name := range []string{"app.css", "app.js"} {
		if _, err := fs.Stat(assets, name); err != nil {
			return nil, fmt.Errorf("ASSET_INIT_FAILURE: %s: %w", name, err)
		}
	}
	return &App{templates: templates, assets: assets, scanner: scanner, runner: runner, mode: "auto", shutdown: shutdown}, nil
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
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(a.assets))))
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
	var output bytes.Buffer
	if err := a.templates.ExecuteTemplate(&output, "page", pageData{title, body, status}); err != nil {
		http.Error(w, "TEMPLATE_RENDER_FAILURE", 500)
		return
	}
	_, _ = w.Write(output.Bytes())
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
	a.mu.RLock()
	running := a.running
	a.mu.RUnlock()
	if running {
		http.Error(w, "OPERATION_IN_PROGRESS", http.StatusConflict)
		return
	}
	version := r.FormValue("version")
	if version != "auto" && !supportedVersion(version) {
		http.Error(w, "INVALID_REQUEST: неподдерживаемая версия", http.StatusBadRequest)
		return
	}
	scan, err := a.scanner.Scan(r.FormValue("path"))
	if err != nil {
		a.render(w, "Ошибка каталога", template.HTML(`<p class="error">`+template.HTMLEscapeString(err.Error())+`</p><a href="/">Назад</a>`), err.Error())
		return
	}
	mode := "auto"
	var selected *string
	if value := version; value != "auto" {
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

// supportedVersion сообщает, известна ли версия реестру диагностик.
func supportedVersion(value string) bool {
	for _, version := range diagnostics.SupportedVersions() {
		if string(version) == value {
			return true
		}
	}
	return false
}
func (a *App) files(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	a.mu.RLock()
	scan := a.scan
	mode, selected := a.mode, a.selected
	a.mu.RUnlock()
	var b strings.Builder
	modeLabel := "Автоопределение"
	if selected != nil {
		modeLabel = *selected
	}
	fmt.Fprintf(&b, `<p>Каталог: <strong>%s</strong></p><p>Режим версии: %s. Сценариев: %d; отклонённых отчётов: %d; ошибок сканирования: %d.</p>`, template.HTMLEscapeString(scan.Root), template.HTMLEscapeString(modeLabel), len(scan.Files), len(scan.RejectedReports), len(scan.Issues))
	b.WriteString(`<form method="post" action="/analysis/all"><button>Проанализировать все</button></form><div class="scroll" role="region" aria-label="Сценарии"><table><caption>Найденные сценарии</caption><thead><tr><th scope="col">Файл</th><th scope="col">Размер</th><th scope="col">Версия DSL</th><th scope="col">Действие</th></tr></thead><tbody>`)
	for _, f := range scan.Files {
		version := f.Version
		if !f.VersionKnown {
			version = "Не определена"
		}
		disabled, explanation := "", ""
		if mode == "explicit" && f.VersionKnown && selected != nil && f.Version != *selected {
			disabled, explanation = " disabled", "Исключён выбором версии"
		}
		fmt.Fprintf(&b, `<tr><td>%s<br><small>%s</small></td><td>%d</td><td>%s</td><td><form method="post" action="/analysis/file"><input type="hidden" name="id" value="%s"><button%s>Проанализировать</button> %s</form>`, template.HTMLEscapeString(f.Name), template.HTMLEscapeString(f.Path), f.Size, template.HTMLEscapeString(version), template.HTMLEscapeString(f.ID), disabled, explanation)
		if f.ReportID != "" {
			fmt.Fprintf(&b, `<a href="/report/%s">Открыть отчёт</a><br><small>Проверен: %s</small>`, template.HTMLEscapeString(f.ReportID), f.ReportTime.Format("02.01.2006 15:04:05"))
		}
		b.WriteString(`</td></tr>`)
	}
	b.WriteString(`</tbody></table></div>`)
	if len(scan.RejectedReports) > 0 {
		b.WriteString(`<div class="scroll" role="region" aria-label="Отклонённые отчёты"><table><caption>Отклонённые и конфликтные отчёты</caption><thead><tr><th scope="col">Путь</th><th scope="col">Код</th><th scope="col">Причина</th></tr></thead><tbody>`)
		for _, issue := range scan.RejectedReports {
			fmt.Fprintf(&b, `<tr><td>%s</td><td>%s</td><td>%s</td></tr>`, template.HTMLEscapeString(issue.Path), template.HTMLEscapeString(issue.Code), template.HTMLEscapeString(issue.Message))
		}
		b.WriteString(`</tbody></table></div>`)
	}
	a.render(w, "Сценарии", template.HTML(b.String()), fmt.Sprintf("Найдено: %d", len(scan.Files)))
}
func (a *App) start(w http.ResponseWriter, r *http.Request, indexes []int) {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		http.Error(w, "OPERATION_IN_PROGRESS", 409)
		return
	}
	if len(a.scan.Files) == 0 {
		a.mu.Unlock()
		http.Error(w, "INVALID_REQUEST: рабочий каталог не выбран или пуст", http.StatusBadRequest)
		return
	}
	files := make([]workspace.File, len(indexes))
	for i, index := range indexes {
		files[i] = a.scan.Files[index]
	}
	mode := a.mode
	var selected *string
	if a.selected != nil {
		value := *a.selected
		selected = &value
	}
	a.running = true
	a.cancel = false
	a.progress = Progress{Total: len(indexes), Remaining: len(indexes), Event: "Операция начата"}
	a.mu.Unlock()
	go a.run(files, mode, selected)
	http.Redirect(w, r, "/progress", http.StatusSeeOther)
}
func (a *App) run(files []workspace.File, mode string, selected *string) {
	for _, file := range files {
		a.mu.Lock()
		if a.cancel {
			a.progress.Cancelled = true
			a.progress.Event = "Операция отменена"
			a.mu.Unlock()
			break
		}
		a.progress.Current = file.Name
		a.progress.Event = "Обрабатывается " + file.Name
		a.mu.Unlock()
		result, err := a.runner.Run(file.Path, mode, selected)
		a.mu.Lock()
		a.progress.Processed++
		a.progress.Remaining--
		if err != nil {
			if errors.Is(err, pipeline.ErrVersionExcluded) {
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
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "INVALID_REQUEST", http.StatusBadRequest)
		return
	}
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
	a.mu.RLock()
	progress := a.progress
	a.mu.RUnlock()
	body := fmt.Sprintf(`<section class="card"><p data-progress>Обработано %d из %d; осталось %d. Текущий файл: %s.</p><dl><dt>Без ошибок</dt><dd>%d</dd><dt>С ошибками</dt><dd>%d</dd><dt>Не завершено</dt><dd>%d</dd><dt>Исключено</dt><dd>%d</dd></dl><form method="post" action="/analysis/cancel"><button>Отменить после текущего файла</button></form><a href="/files">Вернуться к списку</a></section>`, progress.Processed, progress.Total, progress.Remaining, template.HTMLEscapeString(progress.Current), progress.Completed, progress.CompletedWithErrors, progress.Incomplete, progress.Excluded)
	a.render(w, "Ход анализа", template.HTML(body), progress.Event)
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
	if !a.running {
		a.mu.Unlock()
		http.Error(w, "INVALID_REQUEST: операция не выполняется", http.StatusBadRequest)
		return
	}
	a.cancel = true
	a.mu.Unlock()
	http.Redirect(w, r, "/progress", 303)
}
func (a *App) reportPage(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/report/")
	a.mu.RLock()
	path := ""
	for _, file := range a.scan.Files {
		if file.ReportID == id {
			path = file.ReportPath
			break
		}
	}
	a.mu.RUnlock()
	if id == "" || path == "" {
		http.Error(w, "INVALID_REQUEST: отчёт не найден", http.StatusBadRequest)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "REPORT_READ_FAILURE", http.StatusInternalServerError)
		return
	}
	value, err := report.DecodeStrict(data)
	if err != nil {
		http.Error(w, "REPORT_SCHEMA_INVALID", http.StatusInternalServerError)
		return
	}
	entries := filterEntries(value.Entries, r.URL.Query().Get("level"), r.URL.Query().Get("source"), r.URL.Query().Get("category"))
	sortReportEntries(entries, r.URL.Query().Get("sort"), r.URL.Query().Get("direction"))
	var b strings.Builder
	fmt.Fprintf(&b, `<p>Файл: <strong>%s</strong><br>Путь: %s<br>Версия DSL: %s<br>Статус: %s<br>Завершён: %s; длительность: %d мс<br>Записей: %d, ошибок: %d.</p>`, template.HTMLEscapeString(value.SourceFile.Name), template.HTMLEscapeString(value.SourceFile.Path), reportVersion(value), template.HTMLEscapeString(value.Analysis.Status), value.Analysis.FinishedAt.Format(time.RFC3339), value.Analysis.DurationMS, value.Summary.Total, value.Summary.ByLevel.Error)
	b.WriteString(`<form method="get"><label>Уровень <select name="level"><option value="">Все</option><option value="error">Ошибка</option><option value="warning">Предупреждение</option><option value="recommendation">Рекомендация</option></select></label><label>Источник <select name="source"><option value="">Все</option><option value="parser">Парсер</option><option value="diagnostic">Диагностика</option><option value="validator">Валидатор</option></select></label><label>Категория <input name="category"></label><label>Сортировка <select name="sort"><option value="number">Номер</option><option value="code">Код</option><option value="level">Уровень</option><option value="source">Источник</option><option value="category">Категория</option><option value="location">Позиция</option></select></label><label>Направление <select name="direction"><option value="asc">По возрастанию</option><option value="desc">По убыванию</option></select></label><button id="apply">Применить</button></form>`)
	b.WriteString(`<div class="scroll" role="region" aria-label="Записи отчёта"><table><caption>Записи отчёта</caption><thead><tr><th scope="col">№</th><th scope="col">Код</th><th scope="col">Уровень</th><th scope="col">Источник</th><th scope="col">Категория</th><th scope="col">Название</th><th scope="col">Сообщение и описание</th><th scope="col">Основание</th><th scope="col">Позиция</th><th scope="col">Контекст</th><th scope="col">Эталон</th><th scope="col">Неудачный пример</th></tr></thead><tbody>`)
	for _, entry := range entries {
		fmt.Fprintf(&b, `<tr><td>%d</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s<br>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`, entry.Number, esc(entry.Code), esc(string(entry.Level)), esc(string(entry.Source)), nullable(entry.Category), nullable(entry.Title), esc(entry.Message), nullable(entry.Description), nullable(entry.Basis), formatLocation(entry.Location), formatContext(entry.Context), nullable(entry.Reference), nullable(entry.BadExample))
	}
	b.WriteString(`</tbody></table></div><a href="/files">К списку</a>`)
	a.render(w, "Отчёт", template.HTML(b.String()), fmt.Sprintf("Показано записей: %d", len(entries)))
}

// reportVersion возвращает отображаемую версию отчёта.
func reportVersion(value report.Report) string {
	if value.DSLVersion.Canonical == nil {
		return "Не определена"
	}
	return template.HTMLEscapeString(*value.DSLVersion.Canonical)
}

// esc экранирует строку для безопасной сборки HTML-фрагмента.
func esc(value string) string { return template.HTMLEscapeString(value) }

// nullable отображает необязательное поле отчёта.
func nullable(value *string) string {
	if value == nil {
		return "Не применяется"
	}
	return esc(*value)
}

// formatLocation отображает начало и включительный конец диапазона.
func formatLocation(value *report.Location) string {
	if value == nil {
		return "Позиция не определена"
	}
	end := value.End
	if value.Start.Line == end.Line && end.Column > value.Start.Column {
		end.Column--
	}
	return fmt.Sprintf("%d:%d–%d:%d", value.Start.Line, value.Start.Column, end.Line, end.Column)
}

// formatContext отображает контекст с сохранением строк.
func formatContext(value *report.Context) string {
	if value == nil {
		return "Не применяется"
	}
	var b strings.Builder
	b.WriteString(`<details><summary>Показать</summary><pre>`)
	for _, line := range value.Lines {
		fmt.Fprintf(&b, "%d: %s\n", line.Number, esc(line.Text))
	}
	b.WriteString(`</pre></details>`)
	return b.String()
}

// filterEntries применяет независимые фильтры отчёта.
func filterEntries(entries []report.Entry, level, source, category string) []report.Entry {
	out := make([]report.Entry, 0, len(entries))
	for _, entry := range entries {
		if level != "" && string(entry.Level) != level || source != "" && string(entry.Source) != source || category != "" && (entry.Category == nil || *entry.Category != category) {
			continue
		}
		out = append(out, entry)
	}
	return out
}

// sortReportEntries сортирует копию записей, сохраняя номер как последний ключ.
func sortReportEntries(entries []report.Entry, key, direction string) {
	if key == "" {
		key = "number"
	}
	desc := direction == "desc"
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		missingA, missingB := entrySortMissing(a, key), entrySortMissing(b, key)
		if missingA != missingB {
			return !missingA
		}
		cmp := compareEntry(a, b, key)
		if cmp == 0 {
			return a.Number < b.Number
		}
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

// entrySortMissing сообщает отсутствие сортируемого значения.
func entrySortMissing(entry report.Entry, key string) bool {
	switch key {
	case "code":
		return entry.Code == ""
	case "category":
		return entry.Category == nil || *entry.Category == ""
	case "location":
		return entry.Location == nil
	}
	return false
}

// compareEntry сравнивает заполненные значения, оставляя отсутствующие последними.
func compareEntry(a, b report.Entry, key string) int {
	var av, bv string
	switch key {
	case "code":
		av, bv = a.Code, b.Code
	case "level":
		av, bv = string(a.Level), string(b.Level)
	case "source":
		av, bv = string(a.Source), string(b.Source)
	case "category":
		if a.Category != nil {
			av = *a.Category
		}
		if b.Category != nil {
			bv = *b.Category
		}
	case "location":
		if a.Location != nil {
			av = fmt.Sprintf("%09d:%09d", a.Location.Start.Line, a.Location.Start.Column)
		}
		if b.Location != nil {
			bv = fmt.Sprintf("%09d:%09d", b.Location.Start.Line, b.Location.Start.Column)
		}
	default:
		if a.Number < b.Number {
			return -1
		}
		if a.Number > b.Number {
			return 1
		}
		return 0
	}
	if av == "" && bv != "" {
		return 1
	}
	if av != "" && bv == "" {
		return -1
	}
	return strings.Compare(strings.ToLower(av), strings.ToLower(bv))
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

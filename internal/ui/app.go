package ui

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"pgak/internal/config"
	"pgak/internal/db"
	"pgak/internal/history"
	"pgak/internal/ui/editor"
)

type Zone int

const (
	ZoneHeader Zone = iota
	ZoneEditor
	ZoneResults
	zoneCount
)

// modal purposes
const (
	purpConn    = "conn"
	purpDB      = "db"
	purpSchema  = "schema"
	purpHistory = "history"
	purpQuick   = "quick"
	purpTable   = "table"
)

type App struct {
	cfg  *config.Config
	hist *history.History

	client   *db.Client
	connName string
	database string
	schema   string

	databases []string
	schemas   []string

	zone      Zone // selected window
	focused   bool // false = window-selection mode, true = zone has focus
	headerSel int  // 0 connexion, 1 database, 2 schema

	ed  *editor.Editor
	res *Results

	modal       *SelectModal
	modalPurp   string
	tableAction string // pending quick action needing a table
	form        *ConnForm
	showHelp    bool // results help overlay (?)

	initialConn string // --connexion or restored session: connect at startup
	initialDB   string // database to open at startup (restored session)

	width, height int

	status    string
	statusErr bool
	busy      bool
	resInfo   string // "42 lignes · 12ms"
}

func NewApp(cfg *config.Config, hist *history.History) *App {
	a := &App{
		cfg:    cfg,
		hist:   hist,
		zone:   ZoneEditor,
		ed:     editor.New(),
		res:    NewResults(),
		schema: "public",
	}
	return a
}

// SetInitialConnection makes the app connect to the named saved connection
// at startup (--connexion) instead of opening the picker.
func (a *App) SetInitialConnection(name string) { a.initialConn = name }

// RestoreSession restores the last session (connection, database, schema,
// editor content). --connexion, when set, keeps priority for the connection.
func (a *App) RestoreSession(s *config.Session) {
	if s == nil {
		return
	}
	if a.initialConn == "" && s.Connection != "" {
		if _, ok := a.cfg.Get(s.Connection); ok {
			a.initialConn = s.Connection
			a.initialDB = s.Database
		}
	}
	if s.Schema != "" {
		a.schema = s.Schema // kept after connect if still valid
	}
	if s.Query != "" {
		a.ed.SetValue(s.Query)
	}
}

// saveSession persists the current state for the next launch.
func (a *App) saveSession() {
	s := config.Session{
		Connection: a.connName,
		Database:   a.database,
		Schema:     a.schema,
		Query:      a.ed.Value(),
	}
	if err := s.Save(); err != nil {
		log.Printf("session: sauvegarde: %v", err)
	}
}

func (a *App) quit() (tea.Model, tea.Cmd) {
	a.saveSession()
	return a, tea.Quit
}

func (a *App) Init() tea.Cmd {
	// --connexion or restored session: connect right away
	if a.initialConn != "" {
		if c, ok := a.cfg.Get(a.initialConn); ok {
			a.busy = true
			a.setOK("connexion à " + c.Name + "…")
			return connectCmd(c, a.initialDB)
		}
		a.setErr("connexion inconnue : « " + a.initialConn + " »")
	}
	// open the connection selector right away (or the form if none saved)
	if len(a.cfg.Connections) == 0 {
		a.form = NewConnForm(nil)
	} else {
		a.openConnModal()
	}
	return nil
}

// ------------------------------------------------------------ async msgs

type connectedMsg struct {
	client    *db.Client
	databases []string
	schemas   []string
	connName  string
	err       error
}

type queryDoneMsg struct {
	res   *db.Result
	query string
	err   error
}

type tablesMsg struct {
	tables []string
	action string
	err    error
}

func connectCmd(c config.Connection, database string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		log.Printf("connexion: name=%s host=%s port=%d user=%s db=%s sslmode=%s password_env=%s",
			c.Name, c.Host, c.Port, c.User, database, c.SSLMode, c.PasswordEnv)
		if c.PasswordEnv != "" && os.Getenv(c.PasswordEnv) == "" {
			err := fmt.Errorf("la variable d'env « %s » est vide ou non définie — ce champ attend le NOM d'une variable (ex: PGAK_DB_PASSWORD), pas le mot de passe", c.PasswordEnv)
			log.Printf("connexion: %v", err)
			return connectedMsg{connName: c.Name, err: err}
		}
		client, err := db.Connect(ctx, c, database)
		if err != nil {
			log.Printf("connexion: échec: %v", err)
			return connectedMsg{connName: c.Name, err: err}
		}
		dbs, err := client.ListDatabases(ctx)
		if err != nil {
			log.Printf("connexion: liste des bases: %v", err)
			client.Close(ctx)
			return connectedMsg{connName: c.Name, err: err}
		}
		schemas, err := client.ListSchemas(ctx)
		if err != nil {
			log.Printf("connexion: liste des schémas: %v", err)
			client.Close(ctx)
			return connectedMsg{connName: c.Name, err: err}
		}
		log.Printf("connexion: ok (%d bases, %d schémas)", len(dbs), len(schemas))
		return connectedMsg{client: client, databases: dbs, schemas: schemas, connName: c.Name}
	}
}

func queryCmd(client *db.Client, schema, sql string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		if err := client.SetSearchPath(ctx, schema); err != nil {
			log.Printf("requête: set search_path %s: %v", schema, err)
			return queryDoneMsg{query: sql, err: fmt.Errorf("set search_path %s : %w", schema, err)}
		}
		res, err := client.Query(ctx, sql)
		if err != nil {
			log.Printf("requête: échec: %v — sql: %s", err, strings.Join(strings.Fields(sql), " "))
		}
		return queryDoneMsg{res: res, query: sql, err: err}
	}
}

func tablesCmd(client *db.Client, schema, action string) tea.Cmd {
	return func() tea.Msg {
		tables, err := client.ListTables(context.Background(), schema)
		return tablesMsg{tables: tables, action: action, err: err}
	}
}

// ---------------------------------------------------------------- update

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.layout()
		return a, nil

	case connectedMsg:
		a.busy = false
		if msg.err != nil {
			a.setErr(fmt.Sprintf("connexion « %s » : %v", msg.connName, msg.err))
			return a, nil
		}
		if a.client != nil {
			a.client.Close(context.Background())
		}
		a.client = msg.client
		a.connName = msg.connName
		a.database = msg.client.Database
		a.databases = msg.databases
		a.schemas = msg.schemas
		if !contains(a.schemas, a.schema) {
			if contains(a.schemas, "public") {
				a.schema = "public"
			} else if len(a.schemas) > 0 {
				a.schema = a.schemas[0]
			}
		}
		a.updateBanner()
		a.setOK(fmt.Sprintf("connecté à %s/%s", msg.connName, a.database))
		return a, nil

	case queryDoneMsg:
		a.busy = false
		if msg.err != nil {
			a.setErr(msg.err.Error())
			return a, nil
		}
		a.res.SetData(msg.res.Columns, msg.res.Rows)
		a.resInfo = fmt.Sprintf("%d ligne(s) · %s", len(msg.res.Rows), msg.res.Duration.Round(time.Millisecond))
		if !msg.res.IsSelect {
			a.resInfo = fmt.Sprintf("%d ligne(s) affectée(s) · %s", msg.res.Affected, msg.res.Duration.Round(time.Millisecond))
		}
		a.setOK("requête exécutée")
		a.hist.Add(msg.query, a.connName, a.database)
		_ = a.hist.Save()
		a.zone = ZoneResults
		a.focused = true
		a.layout()
		return a, nil

	case tablesMsg:
		a.busy = false
		if msg.err != nil {
			a.setErr(msg.err.Error())
			return a, nil
		}
		items := make([]SelectItem, len(msg.tables))
		for i, t := range msg.tables {
			items[i] = SelectItem{Label: t, Value: t}
		}
		a.tableAction = msg.action
		a.openModal(purpTable, "Tables de "+a.schema, items)
		return a, nil

	case tea.KeyMsg:
		return a.handleKey(msg)
	}
	return a, nil
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if key == "ctrl+c" {
		return a.quit()
	}

	// ----- help overlay: any key closes it
	if a.showHelp {
		a.showHelp = false
		return a, nil
	}

	// ----- form modal
	if a.form != nil {
		if res := a.form.Update(msg); res != nil {
			a.form = nil
			if !res.Canceled {
				a.cfg.Upsert(res.OldName, res.Conn)
				if err := a.cfg.Save(); err != nil {
					a.setErr("sauvegarde config : " + err.Error())
				} else {
					a.setOK("connexion « " + res.Conn.Name + " » enregistrée")
				}
				// reconnect if we edited the active connection
				if res.OldName != "" && res.OldName == a.connName {
					a.busy = true
					return a, connectCmd(res.Conn, a.database)
				}
				if a.client == nil {
					a.openConnModal()
				}
			} else if a.client == nil && a.modal == nil {
				a.openConnModal()
			}
		}
		return a, nil
	}

	// ----- select modal
	if a.modal != nil {
		if res := a.modal.Update(msg); res != nil {
			return a.handleModalResult(res)
		}
		return a, nil
	}

	// ----- global keys
	switch key {
	case "ctrl+e", "f5":
		return a.runQuery()
	case "ctrl+p":
		if a.zone == ZoneEditor || a.zone == ZoneHeader {
			a.openHistoryModal()
			return a, nil
		}
	case "ctrl+t":
		a.openQuickActions()
		return a, nil
	case "ctrl+j":
		a.setZone((a.zone + 1) % zoneCount)
		return a, nil
	case "ctrl+k":
		a.setZone((a.zone + zoneCount - 1) % zoneCount)
		return a, nil
	case "tab", "shift+tab":
		inInsert := a.focused && a.zone == ZoneEditor && a.ed.Mode() == editor.Insert
		if !inInsert {
			if key == "tab" {
				a.setZone((a.zone + 1) % zoneCount)
			} else {
				a.setZone((a.zone + zoneCount - 1) % zoneCount)
			}
			return a, nil
		}
	}

	// ----- window-selection mode: hjkl move, enter focuses
	if !a.focused {
		switch key {
		case "enter", "i", " ", "space":
			a.setFocused(true)
		case "j", "l", "down", "right":
			if a.zone < ZoneResults {
				a.setZone(a.zone + 1)
			}
		case "k", "h", "up", "left":
			if a.zone > ZoneHeader {
				a.setZone(a.zone - 1)
			}
		case "g":
			a.setZone(ZoneHeader)
		case "G":
			a.setZone(ZoneResults)
		case "1":
			a.setZone(ZoneHeader)
		case "2":
			a.setZone(ZoneEditor)
		case "3":
			a.setZone(ZoneResults)
		case "?":
			if a.zone == ZoneResults {
				a.showHelp = true
			}
		case "q", "esc":
			if key == "q" {
				return a.quit()
			}
		}
		return a, nil
	}

	// ----- focused zone: esc (in normal mode) returns to window selection
	if key == "esc" {
		switch a.zone {
		case ZoneHeader:
			a.setFocused(false)
			return a, nil
		case ZoneEditor:
			if a.ed.Mode() == editor.Normal && a.ed.PendingKeys() == "" {
				a.setFocused(false)
				return a, nil
			}
		case ZoneResults:
			if a.res.PendingKeys() == "" {
				a.setFocused(false)
				return a, nil
			}
		}
	}

	// q quits from the results zone (vim-ish habit)
	if key == "q" && a.zone == ZoneResults && a.res.PendingKeys() == "" {
		return a.quit()
	}

	// ? shows the results help (navigation + copy)
	if key == "?" && a.zone == ZoneResults && a.res.PendingKeys() == "" {
		a.showHelp = true
		return a, nil
	}

	// ----- zone dispatch
	switch a.zone {
	case ZoneHeader:
		a.handleHeaderKey(key)
	case ZoneEditor:
		a.ed.Update(msg)
	case ZoneResults:
		a.res.Update(msg)
		if a.res.Status != "" {
			a.setOK(a.res.Status)
		}
	}
	return a, nil
}

func (a *App) setZone(z Zone) {
	a.zone = z
	a.layout()
}

func (a *App) setFocused(b bool) {
	a.focused = b
	a.layout()
}

func (a *App) handleHeaderKey(key string) {
	switch key {
	case "h", "left", "b":
		if a.headerSel > 0 {
			a.headerSel--
		}
	case "l", "right", "w":
		if a.headerSel < 2 {
			a.headerSel++
		}
	case "0":
		a.headerSel = 0
	case "$":
		a.headerSel = 2
	case "enter", "space", " ", "i":
		switch a.headerSel {
		case 0:
			a.openConnModal()
		case 1:
			a.openDBModal()
		case 2:
			a.openSchemaModal()
		}
	}
}

// ------------------------------------------------------------- run query

func (a *App) runQuery() (tea.Model, tea.Cmd) {
	if a.client == nil {
		a.setErr("aucune connexion active (zone connexion → ↵)")
		return a, nil
	}
	sql := strings.TrimSpace(a.ed.Value())
	if sql == "" {
		a.setErr("requête vide")
		return a, nil
	}
	a.busy = true
	a.setOK("exécution…")
	return a, queryCmd(a.client, a.schema, sql)
}

func (a *App) runSQL(sql string) (tea.Model, tea.Cmd) {
	a.ed.SetValue(sql)
	return a.runQuery()
}

// --------------------------------------------------------------- modals

func (a *App) openModal(purpose, title string, items []SelectItem) {
	m := NewSelectModal(title, items)
	m.SetSize(a.modalWidth(), a.modalHeight())
	a.modal = m
	a.modalPurp = purpose
}

func (a *App) modalWidth() int  { return clampInt(a.width-20, 30, 90) }
func (a *App) modalHeight() int { return clampInt(a.height-8, 8, 24) }

func (a *App) openConnModal() {
	items := make([]SelectItem, len(a.cfg.Connections))
	for i, c := range a.cfg.Connections {
		items[i] = SelectItem{
			Label: c.Name,
			Desc:  fmt.Sprintf("%s@%s:%d", c.User, c.Host, defaultPort(c.Port)),
			Value: c.Name,
		}
	}
	a.openModal(purpConn, "Connexions", items)
	a.modal.Actions = map[string]string{"e": "éditer", "n": "nouvelle", "x": "supprimer"}
}

func (a *App) openDBModal() {
	if a.client == nil {
		a.setErr("connectez-vous d'abord")
		return
	}
	items := make([]SelectItem, len(a.databases))
	for i, d := range a.databases {
		items[i] = SelectItem{Label: d, Value: d}
	}
	a.openModal(purpDB, "Bases de données", items)
}

func (a *App) openSchemaModal() {
	if a.client == nil {
		a.setErr("connectez-vous d'abord")
		return
	}
	items := make([]SelectItem, len(a.schemas))
	for i, s := range a.schemas {
		items[i] = SelectItem{Label: s, Value: s}
	}
	a.openModal(purpSchema, "Schémas", items)
}

func (a *App) openHistoryModal() {
	items := make([]SelectItem, 0, len(a.hist.Entries))
	for _, e := range a.hist.Entries {
		label := strings.Join(strings.Fields(e.Query), " ")
		items = append(items, SelectItem{
			Label: label,
			Desc:  e.Connection + "/" + e.Database,
			Value: e.Query,
		})
	}
	a.openModal(purpHistory, "Historique des requêtes", items)
}

func (a *App) openQuickActions() {
	items := []SelectItem{
		{Label: "Rechercher une table", Desc: "SELECT * … LIMIT 100", Value: "table:select"},
		{Label: "Colonnes d'une table", Desc: "types, null, défauts", Value: "table:columns"},
		{Label: "Compter les lignes d'une table", Desc: "SELECT count(*)", Value: "table:count"},
		{Label: "Index d'une table", Desc: "pg_indexes", Value: "table:indexes"},
		{Label: "Lister les tables du schéma", Desc: a.schema, Value: "sql:tables"},
		{Label: "Taille des tables", Desc: "pg_total_relation_size", Value: "sql:sizes"},
		{Label: "Connexions actives", Desc: "pg_stat_activity", Value: "sql:activity"},
		{Label: "Requêtes lentes en cours", Desc: "> 5 secondes", Value: "sql:slow"},
		{Label: "Éditer les connexions", Desc: "config " + configPathHint(), Value: "connections"},
	}
	a.openModal(purpQuick, "Quick actions", items)
}

func configPathHint() string {
	p, err := config.Path()
	if err != nil {
		return ""
	}
	return p
}

func (a *App) handleModalResult(res *SelectResult) (tea.Model, tea.Cmd) {
	purpose := a.modalPurp
	a.modal = nil
	a.modalPurp = ""

	if res.Canceled {
		return a, nil
	}

	switch purpose {
	case purpConn:
		switch res.Action {
		case "e":
			if res.HasItem {
				if c, ok := a.cfg.Get(res.Item.Value.(string)); ok {
					a.form = NewConnForm(&c)
					a.form.SetSize(a.modalWidth(), a.modalHeight())
				}
			}
		case "n":
			a.form = NewConnForm(nil)
			a.form.SetSize(a.modalWidth(), a.modalHeight())
		case "x":
			if res.HasItem {
				name := res.Item.Value.(string)
				a.cfg.Delete(name)
				if err := a.cfg.Save(); err != nil {
					a.setErr("sauvegarde config : " + err.Error())
				} else {
					a.setOK("connexion « " + name + " » supprimée")
				}
				a.openConnModal()
			}
		default:
			if res.HasItem {
				name := res.Item.Value.(string)
				if c, ok := a.cfg.Get(name); ok {
					a.busy = true
					a.setOK("connexion à " + name + "…")
					return a, connectCmd(c, "")
				}
			}
		}

	case purpDB:
		if res.HasItem {
			dbName := res.Item.Value.(string)
			if c, ok := a.cfg.Get(a.connName); ok {
				a.busy = true
				a.setOK("connexion à " + dbName + "…")
				return a, connectCmd(c, dbName)
			}
		}

	case purpSchema:
		if res.HasItem {
			a.schema = res.Item.Value.(string)
			a.updateBanner()
			a.setOK("schéma : " + a.schema)
		}

	case purpHistory:
		if res.HasItem {
			a.ed.SetValue(res.Item.Value.(string))
			a.zone = ZoneEditor
			a.setFocused(true)
		}

	case purpQuick:
		if res.HasItem {
			return a.handleQuickAction(res.Item.Value.(string))
		}

	case purpTable:
		if res.HasItem {
			return a.handleTablePicked(res.Item.Value.(string))
		}
	}
	return a, nil
}

func (a *App) handleQuickAction(action string) (tea.Model, tea.Cmd) {
	if action == "connections" {
		a.openConnModal()
		return a, nil
	}
	if a.client == nil {
		a.setErr("connectez-vous d'abord")
		return a, nil
	}
	switch action {
	case "sql:tables":
		return a.runSQL(fmt.Sprintf(
			`SELECT table_name, table_type
FROM information_schema.tables
WHERE table_schema = '%s'
ORDER BY table_name`, a.schema))
	case "sql:sizes":
		return a.runSQL(fmt.Sprintf(
			`SELECT relname AS table,
       pg_size_pretty(pg_total_relation_size(c.oid)) AS total_size,
       pg_size_pretty(pg_relation_size(c.oid)) AS table_size
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = '%s' AND c.relkind = 'r'
ORDER BY pg_total_relation_size(c.oid) DESC`, a.schema))
	case "sql:activity":
		return a.runSQL(
			`SELECT pid, usename, datname, state, query_start, left(query, 80) AS query
FROM pg_stat_activity
WHERE state IS NOT NULL
ORDER BY query_start`)
	case "sql:slow":
		return a.runSQL(
			`SELECT pid, usename, now() - query_start AS duration, left(query, 100) AS query
FROM pg_stat_activity
WHERE state = 'active' AND now() - query_start > interval '5 seconds'
ORDER BY duration DESC`)
	default: // table:* actions need a table picker
		if strings.HasPrefix(action, "table:") {
			a.busy = true
			return a, tablesCmd(a.client, a.schema, strings.TrimPrefix(action, "table:"))
		}
	}
	return a, nil
}

func (a *App) handleTablePicked(table string) (tea.Model, tea.Cmd) {
	qualified := db.QuoteIdent(a.schema) + "." + db.QuoteIdent(table)
	switch a.tableAction {
	case "columns":
		return a.runSQL(db.ColumnsQuery(a.schema, table))
	case "count":
		return a.runSQL("SELECT count(*) FROM " + qualified)
	case "indexes":
		return a.runSQL(fmt.Sprintf(
			`SELECT indexname, indexdef
FROM pg_indexes
WHERE schemaname = '%s' AND tablename = '%s'
ORDER BY indexname`, a.schema, table))
	default: // select
		return a.runSQL("SELECT *\nFROM " + qualified + "\nLIMIT 100")
	}
}

// ---------------------------------------------------------------- status

func (a *App) setOK(s string)  { a.status, a.statusErr = s, false }
func (a *App) setErr(s string) { a.status, a.statusErr = s, true }

// updateBanner reflects the implicit search_path in the editor header line.
func (a *App) updateBanner() {
	if a.client == nil || a.schema == "" {
		a.ed.SetBanner("")
		return
	}
	a.ed.SetBanner(fmt.Sprintf("  SET search_path TO %s;  -- appliqué automatiquement (schéma sélectionné)", a.schema))
}

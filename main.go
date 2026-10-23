package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"pgak/internal/config"
	"pgak/internal/history"
	"pgak/internal/i18n"
	"pgak/internal/ui"
)

func usage() {
	// Load config to determine language for help text
	if cfg, err := config.Load(); err == nil && cfg.Language != "" {
		i18n.SetLanguage(cfg.Language)
	} else {
		i18n.SetLanguage("en") // default to English
	}

	cfgPath, _ := config.Path()
	histPath, _ := config.HistoryPath()
	logPath, _ := config.LogPath()
	sessPath, _ := config.SessionPath()

	fmt.Fprintf(os.Stderr, "%s\n\n", i18n.T("help.title"))
	fmt.Fprintf(os.Stderr, "%s\n", i18n.T("help.usage"))
	fmt.Fprintf(os.Stderr, "%s\n\n", i18n.T("help.usage.cmd"))
	fmt.Fprintf(os.Stderr, "%s\n\n", i18n.T("help.default"))
	fmt.Fprintf(os.Stderr, "%s\n", i18n.T("help.options"))
	fmt.Fprintf(os.Stderr, "%s\n", i18n.T("help.option.connection"))
	fmt.Fprintf(os.Stderr, "%s\n", i18n.T("help.option.connection2"))
	fmt.Fprintf(os.Stderr, "%s\n\n", i18n.T("help.option.help"))
	fmt.Fprintf(os.Stderr, "%s\n", i18n.T("help.files"))
	fmt.Fprintf(os.Stderr, i18n.T("help.file.config")+"\n", cfgPath)
	fmt.Fprintf(os.Stderr, i18n.T("help.file.history")+"\n", histPath)
	fmt.Fprintf(os.Stderr, i18n.T("help.file.session")+"\n", sessPath)
	fmt.Fprintf(os.Stderr, i18n.T("help.file.logs")+"\n\n", logPath)
	fmt.Fprintf(os.Stderr, "%s\n", i18n.T("help.inapp"))
	fmt.Fprintf(os.Stderr, "%s\n", i18n.T("help.inapp.nav"))
	fmt.Fprintf(os.Stderr, "%s\n", i18n.T("help.inapp.esc"))
	fmt.Fprintf(os.Stderr, "%s\n", i18n.T("help.inapp.exec"))
	fmt.Fprintf(os.Stderr, "%s\n", i18n.T("help.inapp.history"))
	fmt.Fprintf(os.Stderr, "%s\n", i18n.T("help.inapp.actions"))
	fmt.Fprintf(os.Stderr, "%s\n", i18n.T("help.inapp.config"))
	fmt.Fprintf(os.Stderr, "%s\n", i18n.T("help.inapp.help"))
	fmt.Fprintf(os.Stderr, "%s\n\n", i18n.T("help.inapp.quit"))
	fmt.Fprintf(os.Stderr, "%s\n", i18n.T("help.passwords"))
}

func setupLog() func() {
	log.SetOutput(io.Discard)
	p, err := config.LogPath()
	if err != nil {
		return func() {}
	}
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return func() {}
	}
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags)
	log.Printf("--- pgak démarré ---")
	return func() { _ = f.Close() }
}

func main() {
	var connName string
	flag.StringVar(&connName, "connexion", "", "connexion enregistrée à utiliser au lancement")
	flag.StringVar(&connName, "c", "", "alias de --connexion")
	flag.Usage = usage
	flag.Parse()

	closeLog := setupLog()
	defer closeLog()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "pgak:", err)
		os.Exit(1)
	}

	// set language (defaults to English if not specified)
	lang := cfg.Language
	if lang == "" {
		lang = "en"
		cfg.Language = "en"
	}
	i18n.SetLanguage(lang)

	if connName != "" {
		if _, ok := cfg.Get(connName); !ok {
			names := make([]string, len(cfg.Connections))
			for i, c := range cfg.Connections {
				names[i] = c.Name
			}
			fmt.Fprintf(os.Stderr, i18n.T("cli.error.unknown_connection")+"\n", connName)
			if len(names) > 0 {
				fmt.Fprintf(os.Stderr, i18n.T("cli.error.available_connections")+"\n", strings.Join(names, ", "))
			} else {
				fmt.Fprintln(os.Stderr, i18n.T("cli.error.no_connections"))
			}
			os.Exit(1)
		}
	}

	hist, err := history.Load()
	if err != nil {
		hist = &history.History{}
	}

	app := ui.NewApp(cfg, hist)
	if connName != "" {
		app.SetInitialConnection(connName)
	}
	// by default, reopen with the state of the last session
	// (connection, database, schema, editor content)
	if sess, err := config.LoadSession(); err == nil {
		app.RestoreSession(sess)
	}
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "pgak:", err)
		os.Exit(1)
	}
}

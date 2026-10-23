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

	"github.com/anmura/pgak/internal/config"
	"github.com/anmura/pgak/internal/history"
	"github.com/anmura/pgak/internal/ui"
)

func usage() {
	cfgPath, _ := config.Path()
	histPath, _ := config.HistoryPath()
	logPath, _ := config.LogPath()
	sessPath, _ := config.SessionPath()
	fmt.Fprintf(os.Stderr, `pgak — TUI pour naviguer simplement dans PostgreSQL (multi-environnements)

Usage :
  pgak [options]

Par défaut, pgak rouvre dans l'état de la dernière fermeture : même connexion,
même base, même schéma, et la requête de l'éditeur est conservée.

Options :
  --connexion <nom>   se connecte au lancement à la connexion enregistrée <nom>
                      (prioritaire sur la session restaurée · alias : -c <nom>)
  --help, -h          affiche cette aide

Fichiers :
  config      %s
  historique  %s
  session     %s
  logs        %s

Dans l'application :
  hjkl / 1 2 3        naviguer entre les fenêtres · ↵ pour prendre le focus
  esc                 revenir au mode fenêtres (en insert : retour au mode normal)
  ctrl+e / F5         exécuter la requête
  ctrl+p              historique des requêtes
  ctrl+t              quick actions (tables, colonnes, tailles, activité…)
  ?                   aide de la fenêtre résultats (navigation, copies)
  q / ctrl+c          quitter

Mots de passe : jamais stockés — le champ « Mot de passe (var env) » attend le
NOM d'une variable d'environnement (ex : PGAK_PROD_PASSWORD).
`, cfgPath, histPath, sessPath, logPath)
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

	if connName != "" {
		if _, ok := cfg.Get(connName); !ok {
			names := make([]string, len(cfg.Connections))
			for i, c := range cfg.Connections {
				names[i] = c.Name
			}
			fmt.Fprintf(os.Stderr, "pgak: connexion inconnue « %s »\n", connName)
			if len(names) > 0 {
				fmt.Fprintf(os.Stderr, "connexions disponibles : %s\n", strings.Join(names, ", "))
			} else {
				fmt.Fprintln(os.Stderr, "aucune connexion enregistrée — lancez pgak pour en créer une")
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

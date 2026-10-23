# pgak

PostgreSQL TUI navigator for multi-environment workflows, 100% keyboard-driven with vim keybindings.

```
╭──────────────────────────────────────────────────────────╮
│ Connection: prod  │  Database: app  │  Schema: public    │  ← selection zone
╰──────────────────────────────────────────────────────────╯
╭─── Query (ctrl+e execute · ctrl+p history) ──────────────╮
│   1 SELECT * FROM users LIMIT 100                        │  ← vim editor
╰──────────────────────────────────────────────────────────╯
╭─── Results · 2 row(s) · 3ms ─────────────────────────────╮
│ id  │ email             │ created_at │                   │  ← results
╰──────────────────────────────────────────────────────────╯
 NORMAL                        tab zones · ctrl+t actions
```

The focused zone automatically expands.

## Installation

```sh
go build -o pgak .
./pgak
```

## Configuration

JSON file (`$XDG_CONFIG_HOME/pgak/config.json`, e.g., `~/Library/Application Support/pgak/config.json` on macOS) — also editable from the TUI (form):

```json
{
  "connections": [
    {
      "name": "local",
      "host": "localhost",
      "port": 5432,
      "user": "postgres",
      "password_env": "",
      "database": "postgres",
      "sslmode": "prefer"
    },
    {
      "name": "staging",
      "host": "staging.example.com",
      "port": 5432,
      "user": "app",
      "password_env": "PGAK_DB_PASSWORD",
      "database": "app",
      "sslmode": "require"
    }
  ]
}
```

Passwords are **never** stored: `password_env` contains the name of an environment variable read at connection time.

## Keybinds

### Window mode (zone selection)

At launch, pgak is in **window mode**: navigate between the three windows with vim keybinds, then "enter" a window.

| Key | Action |
|---|---|
| `h j k l` | move between windows |
| `g` / `G` | first / last window |
| `1` `2` `3` | jump to window n |
| `↵` / `i` | focus the selected window |
| `q` | quit |

Once in a window, `esc` (in normal mode) returns to window mode. In the editor, `esc` unwinds vim modes: insert → normal → window mode.

The selected window has a yellow border; the focused window takes its zone color (blue / red / green) and expands.

### Global

| Key | Action |
|---|---|
| `tab` / `shift+tab` | next / previous window (outside insert mode) |
| `ctrl+j` / `ctrl+k` | next / previous window (everywhere) |
| `ctrl+e` / `F5` | execute query |
| `ctrl+p` | query history (query zone) |
| `ctrl+t` | quick actions menu |
| `ctrl+c` | quit |

### Selection zone (blue)

`h`/`l` to choose connection / database / schema, `↵` opens the select with fuzzy search. In the connections select: `e` edit, `n` new, `x` delete (in normal mode).

### Query editor (red) — vim

- Modes: normal / insert / visual / visual-line
- Movement: `h j k l`, `w b e`, `0 ^ $`, `gg G`, counts (`3w`, `2j`…)
- Operators: `d`, `y`, `c` + motion (`dd`, `dw`, `d$`, `yy`, `cw`, `cc`…)
- Editing: `i a I A o O`, `x`, `D`, `C`, `s`, `r`, `J`, `p P`
- `u` undo, `ctrl+r` redo
- `v` / `V` visual, then `y`/`d`/`c`
- `esc`: insert → normal → window mode (in a modal: normal → close)
- Yanks also go to the system clipboard

### Results (green)

- Navigation: `h j k l`, `0 $`, `gg G`, `ctrl+d` / `ctrl+u`
- Copy (system clipboard):

| Keys | Copy |
|---|---|
| `yc` | cell |
| `yC` | cell with column name |
| `yy` | row (TSV) |
| `yY` | row with headers |
| `yo` | entire column |
| `yO` | column with header |
| `ya` | all results (TSV) |

- `q`: quit

### Modals (selects, form)

Fuzzy search in insert mode; `esc` switches to normal mode (`j`/`k`, `g`/`G`), `esc` again closes, `↵` validates, `/` or `i` restarts search.

## Quick actions (`ctrl+t`)

- Search for a table (`SELECT * … LIMIT 100`)
- Table columns (types, nullability, defaults)
- Count table rows
- Table indexes
- List schema tables
- Table sizes
- Active connections / slow queries
- Edit connections

## History

Executed queries are stored in history (`history.json`, max 500 entries, deduplicated) and reloadable with `ctrl+p` + fuzzy search.

## Logs & troubleshooting

Connections and query errors are logged in `pgak.log` next to the config (e.g., `~/Library/Application Support/pgak/pgak.log` on macOS):

```sh
tail -f ~/Library/Application\ Support/pgak/pgak.log
```

Common pitfalls:
- **Docker container**: check the *mapped* port (`docker ps`), e.g., `0.0.0.0:5434->5432/tcp` → connect on port **5434**, not 5432.
- **Password**: if `password_env` is set but the variable isn't exported in the shell that launches pgak, the connection is refused with an explicit message.

## Tests

```sh
go test ./...                 # unit tests
PGAK_LIVE=1 go test ./...     # + integration tests (local postgres required)
```

---

# pgak (Français)

TUI de navigation PostgreSQL multi-environnements, 100 % pilotable au clavier avec des keybinds vim.

```
╭──────────────────────────────────────────────────────────╮
│ Connexion: prod  │  Base: app  │  Schéma: public         │  ← zone sélection
╰──────────────────────────────────────────────────────────╯
╭─── Requête (ctrl+e exécuter · ctrl+p historique) ────────╮
│   1 SELECT * FROM users LIMIT 100                        │  ← éditeur vim
╰──────────────────────────────────────────────────────────╯
╭─── Résultats · 2 ligne(s) · 3ms ─────────────────────────╮
│ id  │ email             │ created_at │                   │  ← résultats
╰──────────────────────────────────────────────────────────╯
 NORMAL                        tab zones · ctrl+t actions
```

La zone qui a le focus s'agrandit automatiquement.

## Installation

```sh
go build -o pgak .
./pgak
```

## Configuration

Fichier JSON (`$XDG_CONFIG_HOME/pgak/config.json`, soit `~/Library/Application Support/pgak/config.json` sur macOS) — éditable aussi depuis le TUI (formulaire) :

```json
{
  "connections": [
    {
      "name": "local",
      "host": "localhost",
      "port": 5432,
      "user": "postgres",
      "password_env": "",
      "database": "postgres",
      "sslmode": "prefer"
    },
    {
      "name": "staging",
      "host": "staging.example.com",
      "port": 5432,
      "user": "app",
      "password_env": "PGAK_DB_PASSWORD",
      "database": "app",
      "sslmode": "require"
    }
  ]
}
```

Les mots de passe ne sont **jamais** stockés : `password_env` contient le nom d'une variable d'environnement lue à la connexion.

## Keybinds

### Mode fenêtre (sélection de zone)

Au lancement, pgak est en **mode fenêtre** : on se déplace entre les trois fenêtres avec les keybinds vim, puis on « entre » dans une fenêtre.

| Touche | Action |
|---|---|
| `h j k l` | se déplacer entre les fenêtres |
| `g` / `G` | première / dernière fenêtre |
| `1` `2` `3` | aller à la fenêtre n |
| `↵` / `i` | donner le focus à la fenêtre sélectionnée |
| `q` | quitter |

Une fois dans une fenêtre, `esc` (en mode normal) revient au mode fenêtre. Dans l'éditeur, `esc` dépile d'abord les modes vim : insertion → normal → mode fenêtre.

La fenêtre sélectionnée a une bordure jaune ; la fenêtre focus prend la couleur de sa zone (bleu / rouge / vert) et s'agrandit.

### Global

| Touche | Action |
|---|---|
| `tab` / `shift+tab` | fenêtre suivante / précédente (hors mode insertion) |
| `ctrl+j` / `ctrl+k` | fenêtre suivante / précédente (partout) |
| `ctrl+e` / `F5` | exécuter la requête |
| `ctrl+p` | historique des requêtes (zone requête) |
| `ctrl+t` | menu quick actions |
| `ctrl+c` | quitter |

### Zone sélection (bleue)

`h`/`l` pour choisir connexion / base / schéma, `↵` ouvre le select avec recherche floue. Dans le select des connexions : `e` éditer, `n` nouvelle, `x` supprimer (en mode normal).

### Éditeur de requête (rouge) — vim

- Modes : normal / insertion / visuel / visuel-ligne
- Déplacements : `h j k l`, `w b e`, `0 ^ $`, `gg G`, compteurs (`3w`, `2j`…)
- Opérateurs : `d`, `y`, `c` + motion (`dd`, `dw`, `d$`, `yy`, `cw`, `cc`…)
- Édition : `i a I A o O`, `x`, `D`, `C`, `s`, `r`, `J`, `p P`
- `u` undo, `ctrl+r` redo
- `v` / `V` visuel, puis `y`/`d`/`c`
- `esc` : insertion → normal → mode fenêtre (dans une modale : normal → fermeture)
- Les yanks vont aussi dans le presse-papier système

### Résultats (verte)

- Navigation : `h j k l`, `0 $`, `gg G`, `ctrl+d` / `ctrl+u`
- Copie (presse-papier système) :

| Touches | Copie |
|---|---|
| `yc` | cellule |
| `yC` | cellule avec nom de colonne |
| `yy` | ligne (TSV) |
| `yY` | ligne avec en-têtes |
| `yo` | colonne entière |
| `yO` | colonne avec en-tête |
| `ya` | tout le résultat (TSV) |

- `q` : quitter

### Modales (selects, formulaire)

Recherche floue en mode insertion ; `esc` passe en mode normal (`j`/`k`, `g`/`G`), `esc` à nouveau ferme, `↵` valide, `/` ou `i` relance la recherche.

## Quick actions (`ctrl+t`)

- Rechercher une table (`SELECT * … LIMIT 100`)
- Colonnes d'une table (types, nullabilité, défauts)
- Compter les lignes d'une table
- Index d'une table
- Lister les tables du schéma
- Taille des tables
- Connexions actives / requêtes lentes
- Éditer les connexions

## Historique

Les requêtes exécutées sont historisées (`history.json`, 500 entrées max, dédupliquées) et rechargeables avec `ctrl+p` + recherche floue.

## Logs & dépannage

Les connexions et erreurs de requêtes sont tracées dans `pgak.log` à côté de la config (ex. `~/Library/Application Support/pgak/pgak.log` sur macOS) :

```sh
tail -f ~/Library/Application\ Support/pgak/pgak.log
```

Pièges classiques :
- **Container Docker** : vérifier le port *mappé* (`docker ps`), ex. `0.0.0.0:5434->5432/tcp` → se connecter sur le port **5434**, pas 5432.
- **Mot de passe** : si `password_env` est renseigné mais que la variable n'est pas exportée dans le shell qui lance pgak, la connexion est refusée avec un message explicite.

## Tests

```sh
go test ./...                 # tests unitaires
PGAK_LIVE=1 go test ./...     # + tests d'intégration (postgres local requis)
```

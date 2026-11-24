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
      "database": "postgres",
      "sslmode": "prefer"
    },
    {
      "name": "staging",
      "host": "staging.example.com",
      "port": 5432,
      "user": "app",
      "database": "app",
      "sslmode": "require"
    }
  ]
}
```

Passwords are **never** written to this file: type them in the connection form (masked) and they are stored in the OS keychain (macOS Keychain, Secret Service on Linux, Credential Manager on Windows), under the service `pgak`.

For headless/CI use, an optional `"password_env": "PGAK_DB_PASSWORD"` field (JSON only) names an environment variable used when the keychain has no entry for the connection.

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
| `ctrl+e` / `F5` | execute the query (or queries, separated by `;`) |
| `ctrl+p` | query history (query zone) |
| `ctrl+t` | quick actions menu |
| `ctrl+c` | quit |

### Selection zone (blue)

`h`/`l` to choose connection / database / schema, `↵` opens the select with fuzzy search. In the connections select: `e` edit, `n` new, `x` delete (in normal mode).

### Query editor (red) — vim

Modes: normal / insert / replace (`R`) / visual (`v`) / visual-line (`V`). Every motion below works on its own, after an operator (`d3w`, `c)`, `y%`, `>}`…) and in visual mode. Counts combine (`2d3w` deletes 6 words).

| Motions | Keys |
|---|---|
| chars / lines | `h j k l`, arrows, `space` / `backspace`, `+` / `enter` / `-` / `_` (first non-blank of next / previous / nth line) |
| line | `0 ^ $ g_ \|` (`3\|` column 3, `2$` end of next line), `gm` middle |
| words / WORDs | `w b e ge` · `W B E gE` |
| find on line | `f F t T` + char, `;` / `,` repeat |
| buffer | `gg G` (with count: line n), `N%` (percent), `H M L` (screen top / middle / bottom) |
| scroll | `ctrl+d` / `ctrl+u` (half page), `ctrl+f` / `ctrl+b` (page), `zt zz zb` |
| paragraphs / sentences | `{ }` · `( )` |
| brackets | `%` (match), `[(` `[{` `])` `]}` (enclosing unmatched bracket) |
| search | `/` `?` + text + `↵` (smartcase), `n` / `N`, `*` / `#` (word under cursor), `g*` / `g#` |
| marks | `m{a-z}` set, `'{a-z}` (line) / `` `{a-z} `` (exact), `''` / ` `` ` back to the position before the last jump |

| Operators | Keys |
|---|---|
| delete / yank / change | `d` `y` `c` + motion or text object; doubled on lines: `dd yy cc` |
| indent | `>` `<` (`>>`, `3>>`, `>}`) |
| case | `g~` toggle, `gu` lower, `gU` upper (`g~~ guu gUU` on the line), `~` on the char |
| shortcuts | `x X s S D C Y`, `r` + char (`3rx`), `J` / `gJ` join (with count), `p P gp gP` paste (`gp`: cursor after) |
| repeat | `.` repeats the last change (`3.` with a new count), `u` undo, `ctrl+r` redo |

Text objects (after `d y c g~ gu gU` or in visual mode, `i` inner / `a` around): `w W` word / WORD, `s` sentence, `p` paragraph, `" ' `` ` `` quotes, `( ) b`, `{ } B`, `[ ]`, `< >` brackets (multi-line, nesting-aware).

Special cases follow vim: `cw` acts like `ce`, `dw` on the last word of a line keeps the line break, exclusive motions ending in column 1 become linewise (`d}`), `cc`/`S`/`o`/`O` keep the indent.

Insert mode: `ctrl+w` delete the word before the cursor, `ctrl+u` delete to the start of the line, `ctrl+n` completion, `tab` 4 spaces. Entering insert: `i a I A gI o O gi` (`gi`: where insert mode was last left).

Visual mode: motions and text objects extend the selection, `o` swaps the ends, `gv` reselects. Then `y d x c s` · `Y D X C S R` (linewise) · `> <` (with count) · `~ u U` · `J gJ` · `r` + char · `p` (replace with the register, the replaced text goes to the register).

- `esc`: insert → normal → window mode (in a modal: normal → close)
- Yanks also go to the system clipboard
- Not implemented: block visual (`ctrl+v`), named registers, macros, `=`, `:` commands. `ctrl+e`, `ctrl+o`, `ctrl+p`, `ctrl+t`, `ctrl+j`, `ctrl+k` are application shortcuts.

### Results (green)

- Navigation: `h j k l`, `0 $`, `gg G`, `ctrl+d` / `ctrl+u`, `K` / `M` / `J` (top / middle / bottom of screen)
- Tabs: `H` / `L` previous / next result tab (see below)
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
| `yi` / `yI` | current row / all rows as a SQL `INSERT` (table of the current column) |

- Edit a cell (runs an `UPDATE` on the source table):

| Keys | Action |
|---|---|
| `i` / `↵` | edit the current cell (cursor at the end) |
| `c` | change the cell (start from an empty value) |
| `↵` / `esc` | apply / cancel · `ctrl+u` `ctrl+w` `ctrl+k` delete · `←` `→` `home` `end` |

  The cell must come from a table column (not an expression or a view) and the table's primary key column(s) must be in the result: `SELECT id, name FROM users` is editable, `SELECT name FROM users` or `SELECT upper(name) …` is not. The statement is `UPDATE table SET col = $1 WHERE pk = $2 RETURNING col`: the value is sent as text and cast by the server (a bad value is reported, nothing is changed); the displayed cell is refreshed with the stored value. Typing `NULL` stores SQL NULL. Values are displayed in the server's text form, like psql (`{1,2}` for arrays, json as is, `2024-01-02 03:04:05+01` for timestamps), so a displayed value is always a valid input.

- Add or duplicate a row (runs an `INSERT` into the table of the current column):

| Keys | Action |
|---|---|
| `o` | new draft row under the current one |
| `p` | duplicate the current row as a draft (primary key left to its default) |
| `i` / `c` · `x` | fill a draft cell · reset it to the column default |
| `ctrl+s` / `esc` | insert the row / discard the draft |

  Nothing is written until `ctrl+s`: the draft only lives in the grid. Cells left on `default` are omitted from the `INSERT`, so sequences and defaults apply; the inserted row is refreshed with the stored values (`RETURNING`). If the server refuses the row (NOT NULL, unique…), the draft stays so you can fix it. Generated columns and columns from other tables (joins) cannot be filled.

- `q`: quit

#### Multiple queries → tabs

Several statements can be typed in the editor, separated by `;`. They run in order and each result opens in its own tab in the results zone, like browser or neovim tabs:

```
╭─── Results · 2 row(s) · 3ms ─────────────────────────────╮
│ 1:SELECT * FROM users…  2:SELECT count(*) …  3:UPDATE … │  ← tab bar (H / L)
│ id  │ email             │ created_at │                   │
```

- `H` / `L`: previous / next tab (wraps around)
- A failing statement gets a red tab showing the error; the other statements still run. The first failing tab is selected after the run.
- Semicolons inside strings, comments and `$$` bodies are not separators.
- The whole editor text is saved as one history entry.

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
- **Password**: edit the connection (`e`) and type it in the form — it goes to the OS keychain. On Linux this needs a running Secret Service (gnome-keyring, KWallet…); otherwise use `password_env`.

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
      "database": "postgres",
      "sslmode": "prefer"
    },
    {
      "name": "staging",
      "host": "staging.example.com",
      "port": 5432,
      "user": "app",
      "database": "app",
      "sslmode": "require"
    }
  ]
}
```

Les mots de passe ne sont **jamais** écrits dans ce fichier : saisissez-les dans le formulaire de connexion (masqués), ils sont stockés dans le trousseau du système (Trousseau macOS, Secret Service sous Linux, Credential Manager sous Windows), sous le service `pgak`.

Pour un usage headless/CI, un champ optionnel `"password_env": "PGAK_DB_PASSWORD"` (JSON uniquement) désigne une variable d'environnement utilisée quand le trousseau n'a pas d'entrée pour la connexion.

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
| `ctrl+e` / `F5` | exécuter la requête (ou les requêtes, séparées par `;`) |
| `ctrl+p` | historique des requêtes (zone requête) |
| `ctrl+t` | menu quick actions |
| `ctrl+c` | quitter |

### Zone sélection (bleue)

`h`/`l` pour choisir connexion / base / schéma, `↵` ouvre le select avec recherche floue. Dans le select des connexions : `e` éditer, `n` nouvelle, `x` supprimer (en mode normal).

### Éditeur de requête (rouge) — vim

Modes : normal / insertion / remplacement (`R`) / visuel (`v`) / visuel-ligne (`V`). Chaque motion ci-dessous fonctionne seule, après un opérateur (`d3w`, `c)`, `y%`, `>}`…) et en mode visuel. Les compteurs se combinent (`2d3w` supprime 6 mots).

| Motions | Touches |
|---|---|
| caractères / lignes | `h j k l`, flèches, `espace` / `backspace`, `+` / `↵` / `-` / `_` (premier non-blanc de la ligne suivante / précédente / n-ième) |
| ligne | `0 ^ $ g_ \|` (`3\|` colonne 3, `2$` fin de la ligne suivante), `gm` milieu |
| mots / MOTS | `w b e ge` · `W B E gE` |
| recherche sur la ligne | `f F t T` + caractère, `;` / `,` répéter |
| buffer | `gg G` (avec compteur : ligne n), `N%` (pourcentage), `H M L` (haut / milieu / bas de l'écran) |
| défilement | `ctrl+d` / `ctrl+u` (demi-page), `ctrl+f` / `ctrl+b` (page), `zt zz zb` |
| paragraphes / phrases | `{ }` · `( )` |
| parenthèses | `%` (correspondante), `[(` `[{` `])` `]}` (parenthèse englobante non fermée) |
| recherche | `/` `?` + texte + `↵` (smartcase), `n` / `N`, `*` / `#` (mot sous le curseur), `g*` / `g#` |
| marques | `m{a-z}` poser, `'{a-z}` (ligne) / `` `{a-z} `` (exact), `''` / ` `` ` revenir avant le dernier saut |

| Opérateurs | Touches |
|---|---|
| supprimer / copier / changer | `d` `y` `c` + motion ou objet de texte ; doublé sur les lignes : `dd yy cc` |
| indentation | `>` `<` (`>>`, `3>>`, `>}`) |
| casse | `g~` inverser, `gu` minuscules, `gU` majuscules (`g~~ guu gUU` sur la ligne), `~` sur le caractère |
| raccourcis | `x X s S D C Y`, `r` + caractère (`3rx`), `J` / `gJ` joindre (avec compteur), `p P gp gP` coller (`gp` : curseur après) |
| répétition | `.` rejoue le dernier changement (`3.` avec un nouveau compteur), `u` undo, `ctrl+r` redo |

Objets de texte (après `d y c g~ gu gU` ou en visuel, `i` intérieur / `a` autour) : `w W` mot / MOT, `s` phrase, `p` paragraphe, `" ' `` ` `` guillemets, `( ) b`, `{ } B`, `[ ]`, `< >` parenthèses (multi-lignes, imbrication gérée).

Les cas particuliers suivent vim : `cw` agit comme `ce`, `dw` sur le dernier mot d'une ligne garde le retour à la ligne, une motion exclusive qui finit en colonne 1 devient linéaire (`d}`), `cc`/`S`/`o`/`O` conservent l'indentation.

Mode insertion : `ctrl+w` supprime le mot avant le curseur, `ctrl+u` supprime jusqu'au début de la ligne, `ctrl+n` complétion, `tab` 4 espaces. Entrer en insertion : `i a I A gI o O gi` (`gi` : là où l'insertion a été quittée).

Mode visuel : motions et objets de texte étendent la sélection, `o` échange les extrémités, `gv` resélectionne. Puis `y d x c s` · `Y D X C S R` (lignes entières) · `> <` (avec compteur) · `~ u U` · `J gJ` · `r` + caractère · `p` (remplace par le registre, le texte remplacé va dans le registre).

- `esc` : insertion → normal → mode fenêtre (dans une modale : normal → fermeture)
- Les yanks vont aussi dans le presse-papier système
- Non implémenté : visuel par blocs (`ctrl+v`), registres nommés, macros, `=`, commandes `:`. `ctrl+e`, `ctrl+o`, `ctrl+p`, `ctrl+t`, `ctrl+j`, `ctrl+k` sont des raccourcis de l'application.

### Résultats (verte)

- Navigation : `h j k l`, `0 $`, `gg G`, `ctrl+d` / `ctrl+u`, `K` / `M` / `J` (haut / milieu / bas de l'écran)
- Onglets : `H` / `L` onglet de résultat précédent / suivant (voir ci-dessous)
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
| `yi` / `yI` | ligne courante / toutes les lignes en `INSERT` SQL (table de la colonne courante) |

- Éditer une cellule (exécute un `UPDATE` sur la table d'origine) :

| Touches | Action |
|---|---|
| `i` / `↵` | éditer la cellule courante (curseur à la fin) |
| `c` | changer la cellule (partir d'une valeur vide) |
| `↵` / `esc` | appliquer / annuler · `ctrl+u` `ctrl+w` `ctrl+k` effacer · `←` `→` `home` `end` |

  La cellule doit provenir d'une colonne de table (pas une expression ni une vue) et la ou les colonnes de la clé primaire doivent être dans le résultat : `SELECT id, name FROM users` est éditable, `SELECT name FROM users` ou `SELECT upper(name) …` ne l'est pas. La requête est `UPDATE table SET col = $1 WHERE pk = $2 RETURNING col` : la valeur est envoyée en texte et convertie par le serveur (une valeur invalide est signalée, rien n'est modifié) ; la cellule affichée est rafraîchie avec la valeur enregistrée. Saisir `NULL` enregistre un NULL SQL. Les valeurs sont affichées dans la forme texte du serveur, comme psql (`{1,2}` pour les tableaux, le json tel quel, `2024-01-02 03:04:05+01` pour les timestamps) : une valeur affichée est donc toujours une saisie valide.

- Ajouter ou dupliquer une ligne (lance un `INSERT` dans la table de la colonne courante) :

| Touches | Action |
|---|---|
| `o` | nouvelle ligne en brouillon sous la ligne courante |
| `p` | dupliquer la ligne courante en brouillon (clé primaire laissée à sa valeur par défaut) |
| `i` / `c` · `x` | remplir une cellule du brouillon · la remettre à la valeur par défaut |
| `ctrl+s` / `esc` | insérer la ligne / abandonner le brouillon |

  Rien n'est écrit avant `ctrl+s` : le brouillon ne vit que dans la grille. Les cellules laissées sur `défaut` sont omises de l'`INSERT` (séquences et valeurs par défaut s'appliquent) ; la ligne insérée est rafraîchie avec les valeurs enregistrées (`RETURNING`). Si le serveur refuse la ligne (NOT NULL, unicité…), le brouillon reste pour être corrigé. Les colonnes générées et celles d'autres tables (jointures) ne peuvent pas être remplies.

- `q` : quitter

#### Plusieurs requêtes → onglets

On peut saisir plusieurs requêtes dans l'éditeur, séparées par `;`. Elles s'exécutent dans l'ordre et chaque résultat s'ouvre dans son propre onglet de la zone résultats, comme les tabs d'un navigateur ou de neovim :

```
╭─── Résultats · 2 ligne(s) · 3ms ─────────────────────────╮
│ 1:SELECT * FROM users…  2:SELECT count(*) …  3:UPDATE … │  ← barre d'onglets (H / L)
│ id  │ email             │ created_at │                   │
```

- `H` / `L` : onglet précédent / suivant (en boucle)
- Une requête en échec a un onglet rouge qui affiche l'erreur ; les autres s'exécutent quand même. Le premier onglet en erreur est sélectionné après l'exécution.
- Les `;` dans les chaînes, les commentaires et les corps `$$` ne sont pas des séparateurs.
- Tout le texte de l'éditeur est enregistré comme une seule entrée d'historique.

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
- **Mot de passe** : éditez la connexion (`e`) et saisissez-le dans le formulaire — il part dans le trousseau du système. Sous Linux, il faut un Secret Service actif (gnome-keyring, KWallet…) ; sinon utilisez `password_env`.

## Tests

```sh
go test ./...                 # tests unitaires
PGAK_LIVE=1 go test ./...     # + tests d'intégration (postgres local requis)
```

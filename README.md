# pgak

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
      "user": "anmura",
      "password_env": "",
      "database": "postgres",
      "sslmode": "prefer"
    },
    {
      "name": "prod",
      "host": "prod.example.com",
      "port": 5432,
      "user": "app",
      "password_env": "PGAK_PROD_PASSWORD",
      "database": "app",
      "sslmode": "require"
    }
  ]
}
```

Les mots de passe ne sont **jamais** stockés : `password_env` contient le nom
d'une variable d'environnement lue à la connexion.

## Keybinds

### Mode fenêtre (sélection de zone)

Au lancement, pgak est en **mode fenêtre** : on se déplace entre les trois
fenêtres avec les keybinds vim, puis on « entre » dans une fenêtre.

| Touche | Action |
|---|---|
| `h j k l` | se déplacer entre les fenêtres |
| `g` / `G` | première / dernière fenêtre |
| `1` `2` `3` | aller à la fenêtre n |
| `↵` / `i` | donner le focus à la fenêtre sélectionnée |
| `q` | quitter |

Une fois dans une fenêtre, `esc` (en mode normal) revient au mode fenêtre.
Dans l'éditeur, `esc` dépile d'abord les modes vim : insertion → normal →
mode fenêtre.

La fenêtre sélectionnée a une bordure jaune ; la fenêtre focus prend la
couleur de sa zone (bleu / rouge / vert) et s'agrandit.

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

`h`/`l` pour choisir connexion / base / schéma, `↵` ouvre le select avec
recherche floue. Dans le select des connexions : `e` éditer, `n` nouvelle,
`x` supprimer (en mode normal).

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

Recherche floue en mode insertion ; `esc` passe en mode normal (`j`/`k`,
`g`/`G`), `esc` à nouveau ferme, `↵` valide, `/` ou `i` relance la recherche.

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

Les requêtes exécutées sont historisées (`history.json`, 500 entrées max,
dédupliquées) et rechargeables avec `ctrl+p` + recherche floue.

## Logs & dépannage

Les connexions et erreurs de requêtes sont tracées dans `pgak.log` à côté de
la config (ex. `~/Library/Application Support/pgak/pgak.log` sur macOS) :

```sh
tail -f ~/Library/Application\ Support/pgak/pgak.log
```

Pièges classiques :
- **Container Docker** : vérifier le port *mappé* (`docker ps`), ex.
  `0.0.0.0:5434->5432/tcp` → se connecter sur le port **5434**, pas 5432.
- **Mot de passe** : si `password_env` est renseigné mais que la variable
  n'est pas exportée dans le shell qui lance pgak, la connexion est refusée
  avec un message explicite.

## Tests

```sh
go test ./...                 # tests unitaires
PGAK_LIVE=1 go test ./...     # + tests d'intégration (postgres local requis)
```

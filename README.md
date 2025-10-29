# pellet-tracking

Pellets Tracker est une application Go qui suit vos achats, consommations et stocks de granulés, avec une interface web mobile-first et une API JSON complète.

## Démarrage rapide

```bash
make run
```

L'application écoute par défaut sur [http://127.0.0.1:8080](http://127.0.0.1:8080). Les chemins de données et l'adresse d'écoute sont configurables via les variables d'environnement `PELLETS_DATA_FILE`, `PELLETS_BACKUP_DIR` et `PELLETS_LISTEN_ADDR`.

## Commandes utiles

Un `Makefile` centralise les tâches courantes :

| Commande | Description |
| --- | --- |
| `make build` | Compile le binaire optimisé (`bin/pellets`). |
| `make run` | Compile puis lance le serveur localement. |
| `make test` | Exécute la suite de tests unitaires. |
| `make e2e` | Lance les tests de bout en bout contre le binaire compilé. |
| `make lint` | Télécharge et lance `golangci-lint` sur le projet. |
| `make docker` | Construit l'image Docker minimale. |
| `make clean` | Supprime les artefacts de build (`bin/`). |
| `make tools` | Installe les outils supplémentaires (mockgen). |

Les tests E2E démarrent le binaire Go, simulent des achats/consommations via l'API puis vérifient les pages HTML pour garantir la cohérence des statistiques affichées.

## Image Docker minimale

Le dépôt fournit un `Dockerfile` multi-étapes :

- stage build : compilation Go avec `GOTOOLCHAIN=auto` pour récupérer la version adaptée,
- stage final : image distroless (`gcr.io/distroless/static:nonroot`) contenant uniquement le binaire et un volume `/data` prêt à l'emploi.

Construire l'image :

```bash
make docker
```

L'image expose le port `8080` et définit `PELLETS_DATA_FILE=/data/pellets.json`. Montez un volume persistant pour conserver vos données :

```bash
docker run --rm -p 8080:8080 -v $(pwd)/data:/data pellets-tracker:latest
```

## Intégration continue

Le workflow GitHub Actions `.github/workflows/ci.yml` vérifie chaque push/PR :

1. installation des outils (`golangci-lint`, `mockgen`),
2. linting Go,
3. tests unitaires (`go test ./...`),
4. tests E2E (`go test ./test/e2e`),
5. construction de l'image Docker.

## Synchroniser avec le dépôt canonique

Pour suivre la branche principale du dépôt officiel [kevynb/pellet-tracking](https://github.com/kevynb/pellet-tracking) :

```bash
git remote add origin https://github.com/kevynb/pellet-tracking.git
git fetch origin
```

Ensuite, rebases et pull requests se font sur cette branche `origin/main`.

## Mode TSnet optionnel

Pour exposer l'application sur votre réseau Tailscale, activez TSnet :

```bash
PELLETS_TSNET_ENABLED=1 \
PELLETS_TSNET_AUTHKEY=tskey-... \
PELLETS_TSNET_DIR=/var/lib/pellets-tsnet \
PELLETS_TSNET_HOSTNAME=pellets \
PELLETS_TSNET_LISTEN_ADDR=:443 \
make run
```

Le serveur bascule alors automatiquement sur l'écoute TSnet tout en conservant l'arrêt gracieux.

# CI/CD del propio proyecto (`.github/workflows/`)

A partir del Bloque 17, PipelineGuard usa CI/CD real sobre su propio repositorio:
un workflow de integración continua (`ci.yml`) en cada PR/push, y un workflow de
release (`release.yml`) que corre GoReleaser (ver `docs/goreleaser.md`) cuando se
empuja un tag `v*`.

> ⚠️ **Sin ejecución real verificada.** Este bloque **no** empuja ningún commit,
> PR ni tag — está prohibido correr comandos de git reales en esta sesión (ver
> `docs/action.md` y bloques anteriores para la misma restricción). Lo único
> verificado localmente es que **ambos archivos son YAML sintácticamente válido**
> (parseados con `gopkg.in/yaml.v3`, el mismo método usado para `.goreleaser.yaml`
> y `action.yml` en bloques anteriores). **No hay forma de confirmar que los
> workflows realmente corren y pasan en GitHub Actions sin un push/PR real** —
> eso solo se puede probar empujando este branch (o abriendo un PR) contra el
> repo real, y el workflow de release solo se puede probar empujando un tag `v*`
> de verdad. Ninguna de esas dos cosas se hizo aquí.

## `ci.yml`

**Triggers:** `pull_request` (cualquier rama) y `push` a `main`.

Tres jobs **independientes**, corren en paralelo (ninguno depende de otro vía
`needs:`):

### Job `go`

1. `actions/checkout@v7`.
2. `actions/setup-go@v7` con `go-version-file: go.mod` — lee la versión de Go
   directamente del `go.mod` del repo (`go 1.23` al momento de escribir esto) en
   vez de hardcodearla en el workflow, así que si `go.mod` sube de versión el
   workflow se actualiza solo.
3. `go build ./...`.
4. `go vet ./...`.
5. Verificación de formato: `gofmt -l .` lista los archivos mal formateados; si
   la lista **no está vacía**, el step imprime esos archivos a stderr y hace
   `exit 1` — **falla el job**, no es solo un reporte informativo.
6. `go test ./... -v -count=1`.

### Job `lint`

1. `actions/checkout@v7`.
2. `actions/setup-go@v7` con `go-version-file: go.mod`.
3. `golangci/golangci-lint-action@v9` con `version: v2.13` (versión de la
   herramienta `golangci-lint` en sí, no de la action). **Sin `.golangci.yml`
   custom en este bloque** — corre con la configuración por defecto de
   golangci-lint.

### Job `bash`

1. `actions/checkout@v7`.
2. Instala `shellcheck` (`sudo apt-get install -y shellcheck`) y `bats-core`
   (`sudo npm install -g bats`) en el runner — `ubuntu-latest` no los trae
   listos para el uso que necesitamos (versión pineada de bats vía npm).
   El `npm install -g` necesita `sudo` igual que el `apt-get`: sin él, la
   primera corrida real de este workflow falló con `EACCES` porque el
   usuario del runner no tiene permiso de escritura en el prefix global de
   npm (`/usr/lib/node_modules` o similar) — el runner sí tiene `sudo` sin
   contraseña disponible, como ya lo prueba el `apt-get` de la línea
   anterior.
3. `shellcheck action/scripts/*.sh`.
4. `bats action/scripts/*.bats`.

Este job es el equivalente en CI de lo que hasta ahora solo se corría a mano en
cada bloque de `action/scripts/` (Bloques 12, 13, 15, 16) — ver `docs/action-install.md`,
`docs/action-run.md`, `docs/action-comment.md`, `docs/action-gate.md`.

## `release.yml`

**Trigger:** `push` de un tag que matchea `v*` (p. ej. `v1.0.0`, `v1.2.3`).

Un único job (`goreleaser`):

1. `actions/checkout@v7` con `fetch-depth: 0` — GoReleaser necesita el
   **historial completo** (no un clone superficial) para generar el changelog
   agrupado por commits/PRs desde el tag anterior (`changelog.use: github` en
   `.goreleaser.yaml`); con `fetch-depth` por defecto (1) el changelog saldría
   vacío o incompleto.
2. `actions/setup-go@v7` con `go-version-file: go.mod` — GoReleaser necesita el
   toolchain de Go para compilar la matriz de binarios.
3. `goreleaser/goreleaser-action@v7` con `version: "~> v2"` (instala la última
   versión 2.x de la herramienta GoReleaser, coherente con `version: 2` en
   `.goreleaser.yaml`) y `args: release --clean`.
4. **`Update floating major version tag`**: solo corre si GoReleaser tuvo éxito
   (comportamiento por defecto de los steps). Deriva el major con
   `MAJOR="${GITHUB_REF_NAME%%.*}"` (`v1.0.0` → `v1`, `v0.1.0` → `v0`), crea el tag
   anotado con `git tag -fa "$MAJOR"` sobre el commit del release y lo empuja con
   `git push origin "$MAJOR" --force`. Así `uses: CamiloUrrea/PipelineGuard/action@v1`
   siempre apunta al último release de la serie 1.x, igual que `actions/checkout`
   mantiene su `v4`.
   - Permisos: usa el mismo `contents: write` del job. El push se autentica con el
     `GITHUB_TOKEN` que `actions/checkout` deja configurado (`persist-credentials`
     por defecto).
   - `fetch-depth: 0` en el checkout garantiza que el historial y los tags estén
     completos en el runner.
   - Un push hecho con `GITHUB_TOKEN` **no** dispara nuevos workflows, así que
     mover `v1` no vuelve a lanzar `release.yml` aunque `v1` matchee `v*`.
   - **Guardia de pre-release:** solo un tag **limpio** `^v[0-9]+\.[0-9]+\.[0-9]+$`
     mueve el tag mayor. Un tag con sufijo (`v1.1.0-rc.1`: por convención semver,
     el `-` marca una pre-release; también `v1.2.3+build.5`) imprime
     `Skipping floating tag update: …` y sale con `0`, **sin** ejecutar ningún
     comando de git. Sin esta guardia, publicar una RC apuntaría a todos los
     usuarios de `@v1` a una versión sin terminar. El GitHub Release de ese tag
     lo sigue creando GoReleaser en el step anterior. Verificado localmente
     extrayendo el `run:` del YAML y ejecutándolo con un `git` falso:
     `v1.1.0-rc.1`, `v2.0.0-beta` y `v1.2.3+build.5` → se omite;
     `v1.2.3` / `v1.0.0` / `v0.1.0` → `tag -fa v1|v0` + `push --force`.

### Permisos

```yaml
jobs:
  goreleaser:
    permissions:
      contents: write
```

A nivel de **job**, no de workflow completo — GoReleaser necesita poder crear el
GitHub Release y subir sus artefactos (`contents: write`). El resto de permisos
por defecto del `GITHUB_TOKEN` no se tocan.

### Token

```yaml
env:
  GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Es el token automático que GitHub inyecta en cada workflow run — **no hace falta
crear ni configurar un secret nuevo**. `goreleaser-action` lo necesita en el
entorno para autenticar las llamadas a la API de GitHub (crear el Release, subir
assets, leer PRs/commits para el changelog).

## Por qué los tres jobs de `ci.yml` corren en paralelo

Ninguno depende del resultado de otro: `go` no necesita que `lint` termine para
compilar, y viceversa. Sin `needs:` entre ellos, GitHub Actions los agenda todos
a la vez en runners separados — el feedback de un PR llega en el tiempo del job
**más lento**, no en la suma de los tres. Es el mismo principio que separar
`shellcheck`/`bats` de `go vet`/`go test`: son dominios de falla distintos (Go
vs. bash) y no hay razón para serializarlos.

## Qué falta para probar esto de verdad

- **`ci.yml`**: abrir un PR real (o hacer push a `main`) contra el repo en
  GitHub. Eso confirmaría que los tres jobs efectivamente arrancan, que
  `go-version-file: go.mod` resuelve la versión correcta, que
  `golangci-lint-action@v9` corre sin config custom sin quejarse, y que la
  instalación de `shellcheck`/`bats-core` vía `apt`/`npm` funciona en el runner
  real (aquí solo se validó que el YAML es válido, no que los comandos de
  instalación tengan éxito en `ubuntu-latest`).
- **`release.yml`**: empujar un tag `v*` real. Eso confirmaría que
  `goreleaser-action@v7` con `version: "~> v2"` resuelve e instala GoReleaser,
  que `--clean` + los permisos `contents: write` alcanzan para publicar el
  Release, y que el changelog agrupado (`docs/goreleaser.md`) sale como se
  espera. GoReleaser en sí tampoco está instalado en esta máquina (ver
  `docs/goreleaser.md`), así que ni siquiera se pudo simular localmente con
  `--snapshot`.

Ninguna de las dos cosas se hizo en este bloque — está fuera de su alcance y
prohibido por las reglas del proyecto (no se dispara ni simula una ejecución
real).

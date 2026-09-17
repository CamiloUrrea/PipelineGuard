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
2. Instala `shellcheck` (`apt-get install -y shellcheck`) y `bats-core`
   (`npm install -g bats`) en el runner — `ubuntu-latest` no los trae listos
   para el uso que necesitamos (versión pineada de bats vía npm).
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

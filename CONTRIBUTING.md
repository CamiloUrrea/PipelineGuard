# Cómo contribuir

¡Gracias por tu interés en PipelineGuard! Esta guía resume lo necesario para proponer un cambio. Para vulnerabilidades de seguridad, no abras un issue: sigue [SECURITY.md](SECURITY.md).

## Requisitos

- **Go 1.26** o superior (la versión mínima está en `go.mod`).
- Para los scripts de la Action: **bash**, **[shellcheck](https://github.com/koalaman/shellcheck)**, **[bats-core](https://github.com/bats-core/bats-core)** y **jq** (lo usa `comment.sh` y sus tests).
- **[golangci-lint](https://golangci-lint.run/)** v2 (CI usa la v2.13).

## Correr las verificaciones

Las mismas que corre CI (`.github/workflows/ci.yml`):

```sh
# Go
go build ./...
go vet ./...
gofmt -l .               # no debe listar nada
go test ./... -count=1
golangci-lint run ./...

# Scripts de la Action
shellcheck action/scripts/*.sh
bats action/scripts/*.bats
```

Los tests nunca usan red ni GitHub real: los escáneres se sustituyen por funciones de prueba en Go, y `curl`, `gh` y `pipelineguard` por versiones falsas en `PATH` en los tests bats.

## Commits convencionales

Usa [commits convencionales](https://www.conventionalcommits.org/): `feat:`, `fix:`, `docs:`, `chore:`, `test:`… (con scope opcional, p. ej. `fix(install):`). No es solo estilo: **alimentan el changelog** que GoReleaser genera en cada Release (`.goreleaser.yaml`): `feat` va a *Features*, `fix` a *Fixes*, y `docs` y `chore` se excluyen.

## Fixtures y ejemplos: siempre ficticios

Los fixtures de prueba (p. ej. `internal/parsers/testdata/`) reproducen la **forma** real de la salida de gitleaks y trivy, pero con **valores ficticios**. Nunca incluyas un secreto, token o clave real en un test, un ejemplo, un issue o un PR.

## Documentación

Cada componente tiene su documento en `docs/`. El índice, por bloques, está en [docs/README.md](docs/README.md), y la visión general en [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). Si tu cambio altera un comportamiento documentado, actualiza su documento en el mismo PR.

## Pull requests

Abre el PR contra `main`. La plantilla incluye un checklist (tests, docs, commits convencionales, sin secretos). Los workflows de CI deben quedar en verde.

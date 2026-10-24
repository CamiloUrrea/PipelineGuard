# CI/CD del propio proyecto (`.github/workflows/`)

A partir del Bloque 17, PipelineGuard usa CI/CD real sobre su propio repositorio:
un workflow de integración continua (`ci.yml`) en cada PR/push, y un workflow de
release (`release.yml`) que corre GoReleaser (ver `docs/goreleaser.md`) cuando se
empuja un tag `v*`.

> **Estado de verificación.** Localmente, ambos workflows se validan como YAML
> sintáctico (`gopkg.in/yaml.v3`, el mismo método que para `.goreleaser.yaml` y
> `action.yml`). Además, **sí hubo corridas reales en GitHub Actions**, que
> dejaron hallazgos concretos:
>
> - **`ci.yml`**: la primera corrida real falló en el job `bash` con `EACCES`
>   al instalar bats sin `sudo` (ver "Job `bash`" abajo). El mismo commit que lo
>   corrigió (`fix(ci): handle stdout write errors and add sudo to bats
>   install`) también atendió el manejo del error de escritura a stdout en
>   `cmd/pipelineguard`, el que ahora sale con código `2` (ver `docs/cmd.md`).
> - **`release.yml`**: el tag `v0.1.0` se publicó como GitHub Release, y la
>   composite action instaló sus assets en un workflow real (ver
>   `docs/action.md`).
>
> Lo que **todavía no** corrió de verdad está en "Qué falta verificar en una
> corrida real", al final.

## `ci.yml`

**Triggers:** `pull_request` (cualquier rama) y `push` a `main`.

**Permisos** (Bloque 22), a nivel de workflow:

```yaml
permissions:
  contents: read
```

Ningún job de CI escribe en el repo ni usa la API de GitHub más allá de leer
el código, así que el `GITHUB_TOKEN` queda en solo lectura en vez de heredar los
permisos por defecto del repositorio (que pueden ser de escritura).

Cuatro jobs **independientes**, corren en paralelo (ninguno depende de otro vía
`needs:`):

### Job `go`

1. `actions/checkout@v7`.
2. `actions/setup-go@v7` con `go-version-file: go.mod` — lee la versión de Go
   directamente del `go.mod` del repo (`go 1.26` desde el Bloque 22; antes
   `go 1.23`) en vez de hardcodearla en el workflow, así que si `go.mod` sube de
   versión el workflow se actualiza solo. Se subió a 1.26 porque, a octubre de
   2026, solo Go 1.26 y 1.27 reciben parches de seguridad. `go.mod` no tiene
   línea `toolchain` (`go mod tidy -diff` no la pide).
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
   herramienta `golangci-lint` en sí, no de la action). **No hay `.golangci.yml`
   en el repo**: corre con la configuración por defecto de golangci-lint.

### Job `govulncheck` (Bloque 22)

1. `actions/checkout@v7`.
2. `actions/setup-go@v7` con `go-version-file: go.mod` (mismas versiones de las
   actions que el resto de jobs).
3. `go install golang.org/x/vuln/cmd/govulncheck@latest`.
4. `govulncheck ./...` — falla el job si el código **llama** a una función con
   una vulnerabilidad conocida, en las dependencias o en la librería estándar
   del Go con que se compila. Las vulnerabilidades en paquetes importados pero
   no alcanzables solo se mencionan; no fallan el job.

`@latest` es **deliberado** (y está comentado así en el YAML): el valor de
govulncheck es consultar la base de vulnerabilidades al día, y el job no maneja
secretos (el token es de solo lectura).

Corrido localmente (govulncheck v1.9.0, Go local 1.27.0): `Your code is affected
by 0 vulnerabilities`, exit `0`. Los avisos no alcanzables que reportó eran todos
de la stdlib del Go **local** 1.27.0 (corregidos en 1.27.2/1.27.3), no de las
dependencias del módulo; en CI el resultado depende del parche de Go 1.26 que
instale `setup-go`. Este job todavía no corrió en GitHub.

### Job `bash`

1. `actions/checkout@v7`.
2. Instala `shellcheck` (`sudo apt-get install -y shellcheck`) y `bats-core`
   (`sudo npm install -g bats`) en el runner — `ubuntu-latest` no los trae
   listos. **Ninguna de las dos versiones está pineada**: `apt` instala la de la
   distro y `npm install -g bats` instala la última publicada.
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
     lo sigue creando GoReleaser en el step anterior y, desde el Bloque 22
     (`release.prerelease: auto` en `.goreleaser.yaml`), lo marca como
     **pre-release**, así que tampoco aparece como "Latest" en GitHub. Verificado localmente
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

## Por qué los jobs de `ci.yml` corren en paralelo

Ninguno depende del resultado de otro: `go` no necesita que `lint` termine para
compilar, y viceversa. Sin `needs:` entre ellos, GitHub Actions los agenda todos
a la vez en runners separados — el feedback de un PR llega en el tiempo del job
**más lento**, no en la suma de los tres. Es el mismo principio que separar
`shellcheck`/`bats` de `go vet`/`go test`: son dominios de falla distintos (Go
vs. bash) y no hay razón para serializarlos.

## Qué falta verificar en una corrida real

Con el tag `v1.0.0` ya corrieron de verdad el step de GoReleaser y el step
**`Update floating major version tag`**: la Action fijada como
`CamiloUrrea/PipelineGuard/action@v1` resolvió `v1` a `v1.0.0` en el repo demo
(ver "Estado del proyecto" en el `README.md` de la raíz), lo que también ejercitó
`install.sh` (`resolve_version`) contra la API real de Releases.

Sigue **sin** correr en GitHub:

- **La guardia de pre-release** del step del tag flotante y
  **`prerelease: auto`**: nunca se ha empujado un tag con sufijo
  (`v1.1.0-rc.1`). Localmente, la guardia solo se ejecutó con un `git` falso (ver
  el punto 4 de `release.yml`) y `prerelease: auto` solo pasó `goreleaser check`.
- **Los cambios del Bloque 22 en `ci.yml`**: `permissions: contents: read` y el
  job `govulncheck` (validados solo como YAML y, govulncheck, corriéndolo a mano).

GoReleaser **v2.18.3** se instaló en la máquina de desarrollo en el Bloque 22
(`go install`) y `goreleaser check` pasa (ver `docs/goreleaser.md`). No se corrió
`goreleaser release` (ni siquiera `--snapshot`).

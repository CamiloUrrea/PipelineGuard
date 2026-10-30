# CI/CD del propio proyecto (`.github/workflows/`)

A partir del Bloque 17, PipelineGuard usa CI/CD real sobre su propio repositorio:
un workflow de integración continua (`ci.yml`) en cada PR/push, y un workflow de
release (`release.yml`) que corre GoReleaser (ver `docs/goreleaser.md`) cuando se
empuja un tag `v*`. Desde el Bloque 23 hay además un workflow de *dogfooding*
(`self-scan.yml`, PipelineGuard sobre su propio repo) y Dependabot
(`.github/dependabot.yml`).

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
>   `docs/action.md`). La primera publicación de `v1.1.0` falló por la elección
>   de tag de GoReleaser; ver "Lección de v1.1.0: dos tags en el mismo commit".
>
> Lo verificado con `v1.1.0` y lo que **todavía no** corrió de verdad está en
> "Qué ya se verificó y qué falta en una corrida real", al final.

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
instale `setup-go`. En GitHub ya corrió en verde (ver al final).

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

## `self-scan.yml` — dogfooding (Bloque 23)

Corre PipelineGuard **sobre el propio repositorio**. Es un workflow aparte de
`ci.yml` porque necesita permisos de escritura que CI no tiene.

**Triggers:** `pull_request` y `push` a `main`.

**Concurrencia:** `group: self-scan-${{ github.ref }}` con
`cancel-in-progress: true`: un push nuevo a la misma rama/PR cancela la corrida
anterior en curso.

**Permisos:** `contents: read` a nivel de workflow; el job `pipelineguard`
agrega `pull-requests: write` (comentario del PR) y `security-events: write`
(subida del SARIF).

**Steps:**

1. `actions/checkout@v7`.
2. `actions/setup-go@v7` con `go-version-file: go.mod` (lo necesita el
   `go install` de gitleaks).
3. `go install github.com/zricethezav/gitleaks/v8@v8.30.1`.
4. trivy `v0.72.0` con el script oficial:
   `curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin v0.72.0`.
   El script sale de la rama `main` de trivy; la versión de trivy sí queda fija.
5. `uses: ./action` con `version: v1`.

**Qué prueba y qué no.** `uses: ./action` toma `action.yml` y los scripts de
`action/scripts/` **del checkout del PR**, pero `version: v1` instala el
**último binario publicado** (`v1.x.y` más alto). Por tanto:

- **Sí** prueba los cambios del PR en la Action (instalación, `run.sh`, SARIF,
  comentario, resumen del job, avisos, gate) contra un repo real.
- **No** prueba el código Go del PR: el binario es el de la última release. El
  código Go lo prueba `ci.yml`.
- Pasa `version: v1` porque con una Action local no hay un ref de release del
  que inferir la versión (ver `docs/action.md`, "Qué versión del binario se
  instala").

No hay `.pipelineguard.yml` en el repo, así que corre en **modo informativo**:
los hallazgos (p. ej. si gitleaks marca algún fixture de prueba) nunca hacen
fallar el job; un fallo de la herramienta (exit `2`) sí.

**Validación local:** `actionlint` (v1.7.12) sin hallazgos sobre `self-scan.yml`,
`ci.yml` y `release.yml`. actionlint también lee los inputs de `./action`.

**En GitHub:** el escaneo propio funciona en el repo (verificado en ejecuciones
reales, runner `ubuntu-24.04`).

## Dependabot (`.github/dependabot.yml`, Bloque 23)

`version: 2`, revisión **semanal** de tres entradas:

| Ecosistema | `directory` | Qué actualiza |
|---|---|---|
| `gomod` | `/` | Dependencias de `go.mod` / `go.sum`. |
| `github-actions` | `/` | Actions usadas en `.github/workflows/`. |
| `github-actions` | `/action` | Actions usadas por la composite action (`action/action.yml`: `github/codeql-action/upload-sarif`). |

Los PRs de Dependabot usan `commit-message: { prefix: chore, include: scope }`,
es decir, `chore(deps): …`. Con los regex de `.goreleaser.yaml` (Bloque 23),
esos commits quedan fuera del changelog (ver `docs/goreleaser.md`).

> **Estado:** Dependabot ya funciona en el repo. Lo que sigue **sin confirmar**
> es la entrada `directory: /action` en particular: la documentación oficial de
> Dependabot solo dice que, para `github-actions`, `directory: /` revisa
> `.github/workflows` y un `action.yml` en la **raíz**, y no documenta
> explícitamente un `action.yml` en otro directorio; la entrada `/action` sigue
> la convención habitual para composite actions en subdirectorios. Si Dependabot
> no abre PRs para `action/action.yml` cuando haya una versión nueva de
> `upload-sarif`, revisar esta entrada.

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
   `.goreleaser.yaml`) y `args: release --clean`. En el entorno del step,
   además del token, fija `GORELEASER_CURRENT_TAG: ${{ github.ref_name }}`: así
   GoReleaser publica exactamente el tag que disparó el workflow (ver la
   lección de abajo).
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
     **Verificado también en una ejecución real:** al publicarse `v1.1.0-rc.1`,
     el tag `v1` siguió apuntando a `v1.0.1` (`7e7922d`), comprobado con
     `git ls-remote --tags origin`; y `v1.1.0-rc.1` apareció en la página de
     Releases con la etiqueta *Pre-release*, con `v1.0.1` todavía como *Latest*.

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

### Lección de v1.1.0: dos tags en el mismo commit

La primera publicación de `v1.1.0` **falló**: los tags `v1.1.0` y `v1.1.0-rc.1`
apuntaban al mismo commit, y GoReleaser eligió el rc en vez del tag que había
disparado el workflow. Se corrigió fijando el tag en el step de GoReleaser:

```yaml
env:
  GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  GORELEASER_CURRENT_TAG: ${{ github.ref_name }}
```

`github.ref_name` es el nombre del tag del `push` que lanzó `release.yml`, así
que GoReleaser ya no tiene que deducirlo del commit. Con eso `v1.1.0` quedó
publicada. Ver también `docs/goreleaser.md`.

## Por qué los jobs de `ci.yml` corren en paralelo

Ninguno depende del resultado de otro: `go` no necesita que `lint` termine para
compilar, y viceversa. Sin `needs:` entre ellos, GitHub Actions los agenda todos
a la vez en runners separados — el feedback de un PR llega en el tiempo del job
**más lento**, no en la suma de los tres. Es el mismo principio que separar
`shellcheck`/`bats` de `go vet`/`go test`: son dominios de falla distintos (Go
vs. bash) y no hay razón para serializarlos.

## Qué ya se verificó y qué falta en una corrida real

Con el tag `v1.0.0` ya corrieron de verdad el step de GoReleaser y el step
**`Update floating major version tag`**: la Action fijada como
`CamiloUrrea/PipelineGuard/action@v1` resolvió `v1` a `v1.0.0` en el repo demo,
lo que también ejercitó `install.sh` (`resolve_version`) contra la API real de
Releases.

Con `v1.1.0` (ejecuciones reales, runner `ubuntu-24.04`; ver "Estado del
proyecto" en el `README.md` de la raíz):

- **Release:** `v1.1.0` está publicada y el tag flotante `v1` apunta a ella
  (commit `72518af`), tras corregir la elección de tag de GoReleaser (ver
  "Lección de v1.1.0" arriba).
- **`install.sh`:** resuelve `v1` a `v1.1.0`, descarga
  `pipelineguard_1.1.0_linux_amd64.tar.gz` y verifica el checksum.
- **`github.action_ref` como versión del binario:** con la Action fijada a un
  tag completo (`@v1.1.0-rc.1`) y sin input `version:`, se instaló el binario de
  ese tag.
- **`continue-on-error` en un step `uses:`** de la composite action: verificado
  en una ejecución real con upload-sarif fallando por ruta inexistente (los
  steps siguientes corren, aparece el aviso, el reporte queda en el resumen del
  job y el gate decide). Sin `pull-requests: write`, el comentario falla con
  `403`, aparece el aviso y el job solo falla por el gate.
- **`self-scan.yml`** y **Dependabot** funcionan en el repo.

Sigue **sin** verificarse:

- **La entrada `directory: /action` de Dependabot** en particular (ver arriba).
- Runners Windows, macOS y Ubuntu 26; PRs desde forks; repos privados sin
  GitHub Advanced Security; repos con muchos hallazgos.

Los cambios del Bloque 22 en `ci.yml` (`permissions: contents: read` y el job
`govulncheck`) ya corrieron en verde en GitHub.

GoReleaser **v2.18.3** se instaló en la máquina de desarrollo en el Bloque 22
(`go install`) y `goreleaser check` pasa (ver `docs/goreleaser.md`). No se corrió
`goreleaser release` (ni siquiera `--snapshot`).

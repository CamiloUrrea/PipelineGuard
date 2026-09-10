# Configuración de release (`.goreleaser.yaml`)

[GoReleaser](https://goreleaser.com/) compila PipelineGuard para todas las
plataformas soportadas, genera checksums y un changelog, y publica el GitHub
Release. Este archivo define todo ese flujo.

> ⚠️ **Estado de validación en esta máquina:** GoReleaser **no está instalado**
> aquí (`goreleaser --version` → `command not found`; tampoco está en `~/go/bin`
> ni como shim de scoop). Por eso **no se pudo correr `goreleaser check` ni
> `goreleaser release --snapshot --clean`**. Lo único verificado localmente es
> que el archivo es **YAML sintácticamente válido** (parseado con `gopkg.in/yaml.v3`).
> La semántica del schema y la convención de nombres descritas abajo vienen de la
> documentación de GoReleaser v2, no de una ejecución real.

## Cómo probarlo localmente (cuando GoReleaser esté disponible)

```sh
goreleaser check                          # valida el archivo contra el schema
goreleaser release --snapshot --clean     # build real local, sin tags ni publicar
```

`--snapshot` no requiere un tag de git y no publica nada; `--clean` borra `dist/`
antes de empezar. Los artefactos quedan en `dist/`.

## Secciones del archivo

### `version: 2` / `project_name: pipelineguard`

`version: 2` es el schema actual de GoReleaser (v2). `project_name` alimenta el
`name_template` de los archives (`{{ .ProjectName }}`).

### `builds`

| Campo | Valor | Motivo |
|-------|-------|--------|
| `id` | `pipelineguard` | Identificador interno del build. |
| `main` | `./cmd/pipelineguard` | Paquete `main` del binario (confirmado: `package main` en `cmd/pipelineguard/main.go`). |
| `binary` | `pipelineguard` | Nombre del ejecutable dentro del archive (GoReleaser le añade `.exe` en Windows). |
| `env` | `CGO_ENABLED=0` | Binario estático, sin dependencia de libc; cross-compila sin toolchain de C. |
| `goos` | `linux, darwin, windows` | Las tres plataformas de runners de GitHub Actions. |
| `goarch` | `amd64, arm64` | Intel/AMD y ARM (Apple Silicon, runners ARM). |
| `ldflags` | `-s -w -X main.version={{.Version}}` | `-s -w` quitan tabla de símbolos y DWARF (binario más pequeño). `-X main.version=...` inyecta la versión. |

> **Nota sobre `-X main.version`:** `cmd/pipelineguard/main.go` **todavía no
> declara** una variable `version` (el Bloque 10 no la incluyó y este bloque no
> toca `cmd/`). El linker de Go **ignora silenciosamente** un `-X` a un símbolo
> inexistente, así que esto no rompe el build; simplemente la inyección no tiene
> efecto hasta que un bloque futuro añada `var version = "dev"` a `main`.

La matriz produce **6 binarios**: `{linux,darwin,windows} × {amd64,arm64}`.

### `archives`

- `formats: [tar.gz]` — formato por defecto para todas las plataformas.
- `format_overrides` — Windows usa `zip` en vez de `tar.gz` (convención de la
  plataforma).
- `name_template: {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}` — ver
  "Convención de nombres" abajo.

> `formats`/`format_overrides` como **listas** es sintaxis de GoReleaser **v2.4+**
> (antes era `format:` singular). Si la versión instalada resultara ser anterior a
> v2.4, habría que cambiarlo a `format: tar.gz` y `format_overrides: [{goos: windows, format: zip}]`.

Por defecto GoReleaser incluye en cada archive, además del binario, los archivos
`LICENSE*` y `README*` de la raíz del repo.

### `checksum`

`name_template: 'checksums.txt'` — un único archivo `checksums.txt` con el
SHA-256 de cada archive. El instalador del Bloque 12 lo usará para verificar la
descarga.

### `changelog`

- `sort: asc` — orden cronológico ascendente.
- `use: github` — agrupa por PRs/commits vía la API de GitHub.
- `filters.exclude` — descarta commits `docs:` y `chore:` del changelog.
- `groups` — dos secciones: **Features** (`^feat`) y **Fixes** (`^fix`).

### `release`

`github: { owner: CamiloUrrea, name: PipelineGuard }` — el repo donde se publica
el GitHub Release. **Este bloque no publica nada** (solo modo snapshot local).

## Convención de nombres de los artefactos (CRÍTICO para el Bloque 12)

Con `name_template = {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}`,
`project_name = pipelineguard`, y `.Version` = el tag **sin** la `v` inicial
(p. ej. tag `v1.2.3` → `.Version` = `1.2.3`):

| `.Os` | `.Arch` | Formato | Nombre del artefacto |
|-------|---------|---------|----------------------|
| linux   | amd64 | tar.gz | `pipelineguard_1.2.3_linux_amd64.tar.gz` |
| linux   | arm64 | tar.gz | `pipelineguard_1.2.3_linux_arm64.tar.gz` |
| darwin  | amd64 | tar.gz | `pipelineguard_1.2.3_darwin_amd64.tar.gz` |
| darwin  | arm64 | tar.gz | `pipelineguard_1.2.3_darwin_arm64.tar.gz` |
| windows | amd64 | zip    | `pipelineguard_1.2.3_windows_amd64.zip` |
| windows | arm64 | zip    | `pipelineguard_1.2.3_windows_arm64.zip` |

Más el archivo de checksums: **`checksums.txt`**.

Valores exactos de las variables:

- `.Os` ∈ `{ linux, darwin, windows }` (nota: **`darwin`**, no `macos`).
- `.Arch` ∈ `{ amd64, arm64 }`.

**Dentro** de cada archive: el binario `pipelineguard` (Unix) o
`pipelineguard.exe` (Windows), en la raíz del archive (sin subdirectorio).

El instalador del Bloque 12 tendrá que mapear la salida de `uname -s` / `uname -m`
del runner a estos valores (`Linux`→`linux`, `Darwin`→`darwin`, `x86_64`→`amd64`,
`aarch64`/`arm64`→`arm64`) y descargar
`pipelineguard_<version>_<os>_<arch>.<tar.gz|zip>` desde los assets del release.

### Modo snapshot

`goreleaser release --snapshot` produce nombres **distintos**: `.Version` incluye
un sufijo tipo `-SNAPSHOT-<commit>` (p. ej.
`pipelineguard_1.2.4-SNAPSHOT-abc1234_linux_amd64.tar.gz`). El instalador del
Bloque 12 apunta a los nombres de **release real** de la tabla de arriba, no a los
de snapshot.

## Contenido esperado de `dist/` tras `goreleaser release --snapshot --clean`

> No verificado (GoReleaser no instalado). Esto es lo que la config y la
> documentación de GoReleaser implican:

- Los 6 archives (`pipelineguard_<version>_<os>_<arch>.{tar.gz,zip}`).
- `checksums.txt`.
- `artifacts.json`, `metadata.json`, `config.yaml` (metadatos internos de GoReleaser).
- Un directorio por target con el binario suelto, p. ej.
  `dist/pipelineguard_linux_amd64_v1/pipelineguard`,
  `dist/pipelineguard_windows_arm64/pipelineguard.exe`
  (los targets `amd64` llevan sufijo `_v1` por la versión de microarquitectura GOAMD64).

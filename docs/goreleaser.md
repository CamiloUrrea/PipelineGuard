# Configuración de release (`.goreleaser.yaml`)

[GoReleaser](https://goreleaser.com/) compila PipelineGuard para todas las
plataformas soportadas, genera checksums y un changelog, y publica el GitHub
Release. Este archivo define todo ese flujo.

> ⚠️ **Estado de validación:** GoReleaser **no está instalado** en la máquina
> de desarrollo, así que localmente no se corrió `goreleaser check` ni
> `goreleaser release --snapshot --clean`. Localmente solo se verifica que el
> archivo es **YAML sintácticamente válido** (`gopkg.in/yaml.v3`). La config
> **sí** corre de verdad en CI: `.github/workflows/release.yml` ejecuta
> GoReleaser en cada tag `v*` y publica el GitHub Release (ver `docs/ci-cd.md`).
> La release `v0.1.0` existe, y la Action instaló sus assets en un workflow real
> (ver `docs/action.md`), así que la convención de nombres de abajo ya se
> comprobó en la práctica.

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

> **Nota sobre `-X main.version`:** `cmd/pipelineguard/main.go` declara
> `var version = "dev"` y la usa como `Version` del comando Cobra, así que la
> inyección tiene efecto: `pipelineguard --version` imprime la versión del
> release (ver `docs/cmd.md`). `TestNewRootCmd_VersionIsWired` verifica ese
> cableado. (El linker de Go ignora en silencio un `-X` a un símbolo
> inexistente, así que si alguien renombrara la variable el build no fallaría.
> Ese test es la red de seguridad.)

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
SHA-256 de cada archive. El instalador (`action/scripts/install.sh`) lo usa para
verificar la descarga. **No** hay firma configurada: `checksums.txt` no va
firmado (cosign es roadmap v1.4; ver `docs/action-install.md` para lo que eso
implica).

### `changelog`

- `sort: asc` — orden cronológico ascendente.
- `use: github` — agrupa por PRs/commits vía la API de GitHub.
- `filters.exclude` — descarta commits `docs:` y `chore:` del changelog.
- `groups` — dos secciones: **Features** (`^feat`) y **Fixes** (`^fix`).

### `release`

`github: { owner: CamiloUrrea, name: PipelineGuard }` — el repo donde se publica
el GitHub Release. La publicación la hace `.github/workflows/release.yml` al
empujar un tag `v*`. Localmente solo tiene sentido el modo `--snapshot`, que no
publica nada. No hay `prerelease: auto`: un tag como `v1.1.0-rc.1` se publica
como release normal en GitHub. (El tag flotante y `install.sh` igual lo ignoran,
porque filtran por formato `vX.Y.Z`.)

## Convención de nombres de los artefactos (CRÍTICO para `install.sh`)

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

El instalador (`install.sh`, ver `docs/action-install.md`) mapea la salida de
`uname -s` / `uname -m` del runner a estos valores (`Linux`→`linux`,
`Darwin`→`darwin`, `MINGW*`/`MSYS*`/`CYGWIN*`→`windows`, `x86_64`→`amd64`,
`aarch64`/`arm64`→`arm64`) y descarga
`pipelineguard_<version>_<os>_<arch>.<tar.gz|zip>` desde los assets del release.

### Modo snapshot

`goreleaser release --snapshot` produce nombres **distintos**: `.Version` incluye
un sufijo tipo `-SNAPSHOT-<commit>` (p. ej.
`pipelineguard_1.2.4-SNAPSHOT-abc1234_linux_amd64.tar.gz`). El instalador
apunta a los nombres de **release real** de la tabla de arriba, no a los de
snapshot.

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

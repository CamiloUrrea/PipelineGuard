# Instalación verificada del binario (`action/scripts/install.sh`)

`action/scripts/install.sh` descarga el binario de PipelineGuard correcto para la
plataforma del runner desde **GitHub Releases**, **verifica su checksum SHA-256**,
lo extrae y lo agrega a `$GITHUB_PATH` para que los steps siguientes del workflow
puedan invocar `pipelineguard` directamente.

Lo consume la composite action (`action.yml`, Bloque 13). Este bloque **solo**
escribe y prueba el script; no hay una release real todavía, así que el flujo de
descarga se valida con fixtures locales, no contra GitHub.

## Flujo del script

Ejecutado como `install.sh <version>` (p. ej. `install.sh v1.2.3`, o un major
desnudo como `install.sh v1`, que es el default del input `version`):

1. `require_cmd curl` — falla ruidoso si `curl` no está disponible.
2. `resolve_version <version>` convierte lo pedido en un **tag de release real**
   (ver la sección siguiente). Todo lo que sigue usa ese tag resuelto, **nunca**
   el `version` original. Del tag (`v1.2.3`, mantiene la `v`) se deriva el
   **file_version** (`1.2.3`, sin `v`), porque GoReleaser nombra los assets sin
   la `v` (ver `docs/goreleaser.md`).
3. `detect_os` + `detect_arch` resuelven la plataforma.
4. `archive_name file_version os arch` construye el nombre exacto del asset.
5. Descarga con `curl -fsSL` **dos** archivos a un dir temporal (`mktemp -d`):
   - el archive (`pipelineguard_<version>_<os>_<arch>.tar.gz` / `.zip`)
   - `checksums.txt`
6. `verify_checksum` compara el SHA-256 del archive con su línea en
   `checksums.txt`. **Si no coincide, o no hay línea para ese archivo, aborta.**
7. `extract_binary` descomprime (`tar -xzf` en unix, `unzip` en windows) en
   `<tmp>/bin`.
8. Si `$GITHUB_PATH` está definido (contexto de GitHub Actions), añade `<tmp>/bin`
   a ese archivo; si no, imprime la ruta del binario.

Solo el paso 8, la consulta a la API de `resolve_version` y la descarga tienen
efectos secundarios. Todo lo demás (`detect_os`, `detect_arch`, `archive_name`,
`verify_checksum`, `sha256_hex`) son funciones puras, testeables en aislamiento.

## `resolve_version`: de `v1` a una release real

Un major desnudo como `v1` **no es una GitHub Release**. El tag flotante `v1`
existe en git (lo mueve `release.yml`, ver `docs/ci-cd.md`) para que
`uses: …/action@v1` funcione, pero los **assets** viven en la release exacta
(`v1.2.3`). Por eso `releases/download/v1/pipelineguard_1_…` siempre daría 404.

```bash
resolve_version <requested>
```

- **Versión completa** (cualquier cosa que no sea `^v[0-9]+$`, p. ej. `v0.1.0`)
  → se imprime **tal cual**, sin llamar a la API.
- **Major desnudo** (`^v[0-9]+$`, p. ej. `v1`):
  1. Consulta `https://api.github.com/repos/CamiloUrrea/PipelineGuard/releases?per_page=100`
     con `curl -fsSL` (`fetch_releases_json`). `per_page=100` es el máximo de la
     API; el default de 30 podría quedarse corto.
  2. Extrae cada `"tag_name": "…"` con `grep -o` + `sed`.
  3. Se queda solo con los tags `^<major>\.[0-9]+\.[0-9]+$`. Así `v1` no matchea
     `v10.0.0`, y **las pre-releases (`v1.3.0-rc.1`) nunca se eligen**.
  4. Toma la más alta con `sort -V` (orden de versiones: `v1.10.0` > `v1.9.0`).
  5. Si no hay ninguna, o la llamada a la API falla, **aborta** con un mensaje
     claro. Nunca sigue con un tag adivinado cuya URL sabe que va a fallar.
- Si `GITHUB_TOKEN` está en el entorno, se envía como `Authorization: Bearer`
  (límite de la API más alto). Sin él, la consulta es anónima (60 req/h por IP).
  Hoy el step de instalación de `action.yml` **no** le pasa ese token.

### Por qué `grep`/`sed` y no `jq`

`install.sh` nunca ha dependido de `jq`, y así se mantiene: solo `comment.sh`
lo requiere. El instalador es lo primero que corre en un runner arbitrario, y
cada dependencia extra es un punto más de fallo antes de tener siquiera el
binario. Extraer un único campo string plano (`tag_name`) no justifica un
parser JSON: `grep -o '"tag_name"[[:space:]]*:[[:space:]]*"[^"]*"'` es robusto
ante el formato real de la API (pretty-printed, objetos anidados, otros campos
`*name`), y los tests lo ejercen con una respuesta de esa forma.

### Guard de `BASH_SOURCE`

El pie del script es:

```bash
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
	install_pipelineguard "${1:-}"
fi
```

Así, cuando **bats** hace `source install.sh` para probar las funciones, el flujo
de descarga **no** se dispara. Solo corre cuando el script se ejecuta directamente.

## Mapeo de plataforma

| `uname -s`                    | `detect_os` |
|-------------------------------|-------------|
| `Linux`                       | `linux`     |
| `Darwin`                      | `darwin`    |
| `MINGW*` / `MSYS*` / `CYGWIN*` | `windows`   |
| cualquier otro                | **error**   |

| `uname -m`          | `detect_arch` |
|--------------------|---------------|
| `x86_64` / `amd64` | `amd64`       |
| `aarch64` / `arm64`| `arm64`       |
| cualquier otro     | **error** (nunca un default silencioso) |

Estos valores coinciden exactamente con `.Os` / `.Arch` de GoReleaser
(`docs/goreleaser.md`): nota `darwin`, no `macos`.

## Por qué se verifica el checksum SIEMPRE

El binario se descarga por HTTPS desde `github.com`, pero eso solo garantiza el
transporte. Verificar el SHA-256 contra el `checksums.txt` (que GoReleaser genera
y firma como parte del release) protege contra:

- Una descarga corrupta o truncada.
- Un asset alterado en el release (cuenta comprometida, MITM en un proxy interno,
  cache envenenado).

Es una herramienta de **seguridad**: ejecutar un binario no verificado con acceso
al repo y a `GITHUB_TOKEN` sería exactamente el tipo de riesgo de cadena de
suministro que PipelineGuard existe para detectar. Si el checksum no coincide, el
script termina con código ≠ 0 y un mensaje claro (`expected` vs `actual`) — nunca
continúa.

## Correr shellcheck y bats localmente

```sh
shellcheck action/scripts/install.sh
bats action/scripts/install_test.bats
```

Instalación de las herramientas (si faltan):

- **shellcheck**: `winget install koalaman.shellcheck` (Windows) ·
  `brew install shellcheck` (macOS) · `apt install shellcheck` (Debian/Ubuntu).
- **bats**: `npm install -g bats` · `brew install bats-core` ·
  o clonar `bats-core/bats-core`.

### Qué cubren los tests bats

`install_test.bats` sourcea `install.sh` (sin red) y prueba:

- `detect_os` / `detect_arch` con `uname` sobreescrito por una función de test,
  para `Linux`/`Darwin`/`MINGW*` y `x86_64`/`aarch64`/`arm64`.
- Arquitectura desconocida → error explícito, y se verifica que la salida **no**
  es `amd64` ni `arm64` (no hay default oculto).
- `archive_name` para las 6 combinaciones os/arch, incluyendo el `.zip` de
  windows.
- `verify_checksum`: checksum correcto → éxito silencioso; checksum incorrecto →
  falla con `checksum mismatch`; archivo sin línea en `checksums.txt` → falla con
  `no checksum entry`. Todo con archivos de fixture en `$BATS_TEST_TMPDIR`.
- `resolve_version`, con un `curl` **falso** primero en PATH (nunca la API real).
  Registra cada llamada en un log que sirve de *spy* y responde a la URL de la
  API con un JSON fijo con la forma de la respuesta real:
  - `v1` con `v1.0.0`, `v1.2.3`, `v0.9.0` → `v1.2.3`.
  - `v1.10.0` le gana a `v1.9.0` (`sort -V`, no orden lexicográfico).
  - `v1.3.0-rc.1` y `v10.0.0` nunca se eligen para `v1`.
  - `v1` sin release que coincida → error claro y ningún tag en la salida.
  - La API falla → error claro (`could not query …`).
  - `v0.1.0` → se devuelve igual, y el log del spy confirma que `curl` **no**
    se llamó.
  - Con `GITHUB_TOKEN` seteado se envía `Authorization: Bearer …`.
- Flujo completo (`bash install.sh v1`, como proceso propio con su `set -e`,
  igual que en la Action):
  - La descarga usa `releases/download/v1.2.3/pipelineguard_1.2.3_…`, nunca
    `releases/download/v1/`.
  - Sin release que coincida → falla **antes** de cualquier descarga.

El "camino feliz" completo (descarga real + extracción) **no** se testea aquí:
requiere una release publicada. Se validará en el test de integración de la Action
(bloque futuro).

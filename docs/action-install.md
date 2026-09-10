# Instalación verificada del binario (`action/scripts/install.sh`)

`action/scripts/install.sh` descarga el binario de PipelineGuard correcto para la
plataforma del runner desde **GitHub Releases**, **verifica su checksum SHA-256**,
lo extrae y lo agrega a `$GITHUB_PATH` para que los steps siguientes del workflow
puedan invocar `pipelineguard` directamente.

Lo consume la composite action (`action.yml`, Bloque 13). Este bloque **solo**
escribe y prueba el script; no hay una release real todavía, así que el flujo de
descarga se valida con fixtures locales, no contra GitHub.

## Flujo del script

Ejecutado como `install.sh <version>` (p. ej. `install.sh v1.2.3`):

1. `require_cmd curl` — falla ruidoso si `curl` no está disponible.
2. Separa el **tag** (`v1.2.3`, mantiene la `v`) del **file_version** (`1.2.3`,
   sin `v`) — GoReleaser nombra los assets sin la `v` (ver `docs/goreleaser.md`).
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

Solo el paso 8 y la descarga tienen efectos secundarios. Todo lo demás
(`detect_os`, `detect_arch`, `archive_name`, `verify_checksum`, `sha256_hex`) son
funciones puras/testeable en aislamiento.

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

El "camino feliz" completo (descarga real + extracción) **no** se testea aquí:
requiere una release publicada. Se validará en el test de integración de la Action
(bloque futuro).

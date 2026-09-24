# Comentario en el PR (`action/scripts/comment.sh`)

`action/scripts/comment.sh` publica el reporte Markdown de PipelineGuard como un
comentario en el Pull Request actual, **actualizando su propio comentario
anterior** en vez de crear uno nuevo en cada corrida. Lo consume el step 4 de la
composite action (`action.yml`).

> ⚠️ **Verificación en esta máquina:** ni `gh` ni `jq` estaban instalados. Instalé
> **jq 1.8.2** (`winget install jqlang.jq`) para poder testear `find_comment_id`
> de verdad. **`gh` sigue sin instalarse** para los tests bats (usan un `gh`
> **falso**), pero la validación end-to-end con `gh` real y un PR real sí se hizo
> y encontró un bug: el script usaba `-f body=@archivo` en vez de
> `-F body=@archivo`, así que el PR recibió el texto literal `@/tmp/tmp.XXXXXXXX`
> en lugar del reporte. Ver [Bug real: `-f` vs `-F` en `gh api`](#bug-real-f-vs-f-en-gh-api)
> más abajo.

## El marcador y la lógica update-vs-create

Cada comentario que publicamos empieza con un marcador fijo:

```
<!-- pipelineguard-report -->
```

Es un **comentario HTML**: invisible en el Markdown renderizado, pero presente en
el `body` crudo del comentario. El flujo:

1. Lista los comentarios del PR:
   `gh api repos/{owner}/{repo}/issues/${PR_NUMBER}/comments --paginate`.
2. `find_comment_id` busca con `jq` el **primer** comentario cuyo `body` contiene
   el marcador y echoea su `id` (o nada si no hay match).
3. - Si hay `id` → **PATCH** `repos/{owner}/{repo}/issues/comments/<id>`.
   - Si no → **POST** `repos/{owner}/{repo}/issues/${PR_NUMBER}/comments`.

Así siempre hay **exactamente un** comentario de PipelineGuard por PR, editado en
sitio a medida que se van subiendo commits. Sin el marcador, cada push dejaría un
comentario nuevo y el PR se llenaría de ruido.

> Nota: los comentarios de un PR se listan por el endpoint de **issues**
> (`/issues/{n}/comments`), no el de *pull request review comments*. Un PR es un
> issue con código; su comentario "de conversación" vive ahí.

### `gh api --paginate` y arrays concatenados

`gh api --paginate` sobre un endpoint que devuelve un array emite **varios arrays
JSON concatenados** (`[...][...]`), no un único array. `find_comment_id` usa
`jq -s` (slurp) + `.[] | .[]?` para aplanar tanto ese caso como el de un solo
array o un `[]` vacío. Cubierto por los tests bats.

## Funciones

| Función | Firma | Qué hace |
|---------|-------|----------|
| `require_cmd` | `require_cmd <name>` | Falla ruidoso si el comando falta. Copia local (duplicación aceptada frente a `install.sh` / `run.sh`). |
| `build_comment_body` | `build_comment_body <marker> <report_path>` | Echoea el marcador, una línea en blanco, y el contenido del archivo de reporte. |
| `find_comment_id` | `find_comment_id <comments_json> <marker>` | Con `jq`, echoea el `id` del primer comentario cuyo `body` contiene el marcador, o nada. `comments_json` puede ser `-` para leer de stdin. |

## Flujo principal (detrás del guard `BASH_SOURCE`)

1. `require_cmd gh` y `require_cmd jq`.
2. **`PR_NUMBER` no seteado → error claro y `exit 1`**: sin saber a qué PR
   comentar no hay nada que hacer.
3. Verifica que el archivo de reporte (`REPORT_PATH`, default
   `pipelineguard-report.md`) exista → si no, error y `exit 1`.
4. `export GH_REPO="${GH_REPO:-${GITHUB_REPOSITORY:-}}"` — `gh` resuelve
   `{owner}/{repo}` desde `GH_REPO`; en Actions `GITHUB_REPOSITORY` siempre está,
   así que se usa como fallback.
5. Construye el body en un archivo temporal (`mktemp`) con `build_comment_body`.
   Justo después de crearlo registra `trap 'rm -f "$body_file"' EXIT`, así que el
   archivo se borra **como sea que termine el script**, incluso si una llamada a
   `gh api` falla y `set -e` corta `main()` a la mitad. (Antes el `rm -f` estaba
   al final de `main()` y un fallo de `gh` dejaba el temporal huérfano.)
   `body_file` es una variable **global** a propósito, no `local`: el trap corre
   cuando `main()` ya terminó, y en ese momento una `local` ya no existiría.
6. Lista comentarios → `find_comment_id` → PATCH o POST con `-F body=@<tmp>`.

### Bug real: `-f` vs `-F` en `gh api`

`gh api` tiene dos flags para pasar campos que **no son intercambiables**:

- `-f` / `--raw-field` — envía el valor **tal cual, literal**. El prefijo `@` no
  se interpreta nunca; es un carácter más del string.
- `-F` / `--field` — además de tipar el valor (`true`/`false`/`null`/números),
  es el único que reconoce el prefijo `@archivo` como "lee el contenido de este
  archivo y úsalo como valor".

El bug real detectado en la validación end-to-end: el script usaba
`-f "body=@${body_file}"`. Como `-f` nunca resuelve `@`, el comentario
publicado en el PR contenía literalmente el texto `@/tmp/tmp.XXXXXXXX` en vez
del reporte Markdown. El fix es usar `-F "body=@${body_file}"` en **ambas**
llamadas (PATCH y POST) — no es un detalle de estilo ni de mayúscula/minúscula
arbitraria, es la única flag de `gh api` que sabe leer un archivo con `@`.
Cubierto por un test bats que falla si alguna de las dos llamadas vuelve a usar
`-f` para `body`.

## Por qué el step se restringe a `pull_request`

```yaml
- name: Post or update PR comment
  if: always() && github.event_name == 'pull_request'
```

- **`github.event_name == 'pull_request'`** — solo hay un PR al que comentar en
  eventos de tipo `pull_request`. En un `push` a `main`, un `schedule` o un
  `workflow_dispatch` **no existe** `github.event.pull_request.number`, así que
  `PR_NUMBER` llegaría vacío y el script abortaría de todos modos — la condición
  simplemente evita ejecutar un step que no tiene sentido en ese contexto. (El
  SARIF del step 3 sí se sube siempre; el comentario es específico del PR.)
- **`always()`** — el comentario debe publicarse aunque `pipelineguard` haya
  encontrado una violación (exit 1) o haya fallado (exit 2). El reporte es
  justamente lo que el desarrollador necesita ver en esos casos. Recuerda que
  `run.sh` termina con `exit 0` salvo binario ausente (ver `docs/action-run.md`),
  así que en el camino normal `always()` es redundante, pero se agrega por
  robustez.

## Permisos requeridos en el workflow llamante

```yaml
permissions:
  pull-requests: write   # crear/editar el comentario del PR
  security-events: write # (del step 3) subir el SARIF
  contents: read
```

`GH_TOKEN` se pasa como `${{ github.token }}` (el `GITHUB_TOKEN` del job).

## Correr shellcheck y bats

```sh
shellcheck action/scripts/comment.sh
bats action/scripts/comment_test.bats
```

`comment_test.bats` pone un `gh` **falso** primero en PATH (registra sus
argumentos en un log, anota si el archivo `body=@<tmp>` existía mientras corría,
y devuelve JSON de comentarios fijo o una respuesta de escritura simulada según
se le llame; con `GH_FAKE_FAIL_WRITES=1` falla en PATCH/POST) y usa `jq` real.
Cubre:

- `find_comment_id`: match → id correcto; sin match → vacío; `[]` → vacío sin
  error; arrays concatenados de `--paginate`; lectura desde stdin con `-`.
- `build_comment_body`: empieza con el marcador y contiene el reporte.
- Flujo con "ya existe comentario con marcador" → se llama **PATCH**, no POST.
- Flujo con "no hay comentarios" → se llama **POST**, no PATCH.
- `PR_NUMBER` vacío → error inmediato, `gh` nunca se invoca.
- Archivo de reporte ausente → error claro antes de cualquier escritura con `gh`.
- Regresión del bug `-f` vs `-F`: tanto el flujo de PATCH como el de POST
  verifican que la llamada a `gh api` use `-F body=@<archivo>` y **no**
  `-f body=@<archivo>`.
- Limpieza del temporal (el `trap`): tras una corrida exitosa, y tras un `gh`
  que falla en POST y en PATCH (el script sale con error), el archivo
  `body=@<tmp>` existió durante la llamada y **ya no existe** al terminar.

> **Nota sobre `set -e` en los tests:** `setup()` sourcea `comment.sh`, que
> activa `set -euo pipefail`. Antes el `setup()` hacía `set +e` después, y eso
> desactivaba en silencio todas las aserciones que no fueran la **última línea**
> de cada test (bats necesita `-e` para que cada línea pueda fallar). Ahora solo
> se hace `set +u`. Además, cada `! grep ...` intermedio se escribe
> `! grep ... || false`, porque bash nunca deja que un comando negado con `!`
> dispare `set -e`.

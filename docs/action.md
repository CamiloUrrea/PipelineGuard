# Composite action (`action/action.yml`)

`action/action.yml` es la GitHub composite action que envuelve todo PipelineGuard:
instala el binario, lo ejecuta, y expone el resultado como outputs. Un repo la usa
con una línea:

```yaml
- uses: CamiloUrrea/PipelineGuard/action@v1
```

> ⚠️ **Sin tests automáticos de `action.yml`.** El archivo en sí no tiene pruebas
> unitarias: se valida localmente como YAML sintáctico y con sus cuatro scripts
> (`install.sh`, `run.sh`, `comment.sh`, `gate.sh`), que pasan shellcheck + bats
> por separado (también en CI, ver `docs/ci-cd.md`). La Action **sí se probó
> de verdad** en un workflow real, contra un Pull Request real en
> `pipelineguard-demo-vulnerable-app`, con la release `v0.1.0` publicada. Esa
> validación end-to-end encontró el bug de `-f` vs `-F` en `gh api` (ver
> `docs/action-comment.md`). Se repitió con `v1.0.0` (`action@v1`, ver
> "Estado del proyecto" en el `README.md`). Los cambios del Bloque 23, publicados
> en `v1.1.0`, también corrieron en GitHub (ver el estado de verificación más
> abajo).

**Steps.** Con el Bloque 16 (`gate.sh`) la Action tenía 5 steps. El Bloque 23
(publicado en `v1.1.0`) agrega 2 más, *Add report to job summary* y *Warn about skipped
integrations*, y marca la subida del SARIF y el comentario del PR como
integraciones **opcionales** (`continue-on-error: true`): ahora son **7 steps**,
y el gate sigue siendo el último y el único que decide el resultado del job.

> **Estado de verificación (Bloque 23).** Validado localmente: `action.yml`
> como YAML; los dos `run:` nuevos extraídos y pasados por shellcheck y
> ejecutados a mano con variables de prueba; y actionlint sobre
> `.github/workflows/self-scan.yml`, que lee los inputs de `./action`. La
> documentación oficial ("Metadata syntax for GitHub Actions") lista
> `continue-on-error` e `id` entre las claves de `runs.steps[*]` de una
> composite action, sin decir explícitamente que `continue-on-error` aplique a
> los steps con `uses:` (el de upload-sarif). Eso quedó **verificado en una
> ejecución real** (runner `ubuntu-24.04`): con upload-sarif fallando por una
> ruta inexistente, los steps siguientes se ejecutaron, apareció el aviso del
> step 6 y el reporte quedó en el resumen del job; el resultado lo decidió el
> gate. También se verificó que, sin `pull-requests: write`, el comentario falla
> con `403`, aparece el aviso y el job solo falla por el gate.

## Permisos requeridos en el workflow que la usa

El **job** que invoca la action necesita:

```yaml
permissions:
  security-events: write   # step 3: subir el SARIF a la pestaña Security
  pull-requests: write     # step 4: crear/editar el comentario del PR
  contents: read
```

Una composite action **no puede** conceder permisos por sí misma; es
responsabilidad del workflow llamante.

## Inputs

| Input          | Default                       | Descripción |
|----------------|-------------------------------|-------------|
| `version`      | `""` (vacío; `v1` hasta v1.0.x) | Tag de release del binario a instalar (`v1`, `v1.2.3`, `v1.1.0-rc.1`, …). **Vacío** = sigue a la versión de la Action: el step 1 pasa `inputs.version \|\| github.action_ref` a `install.sh`. Con `@v1` → último `1.x`; con `@v1.1.0` → exactamente `v1.1.0`. Con una rama o un SHA hay que pasar `version:` explícito, o `install.sh` falla pidiéndolo. Ver "Qué versión del binario se instala". |
| `config-path`  | `.pipelineguard.yml`          | Ruta al archivo de configuración. Se pasa a `run.sh` como `CONFIG_PATH`. |
| `sarif-output` | `pipelineguard-results.sarif` | Ruta del reporte SARIF. Se pasa a `run.sh` como `SARIF_OUTPUT`. |

## Outputs

| Output        | Origen                              | Descripción |
|---------------|-------------------------------------|-------------|
| `exit-code`   | `steps.run.outputs.exit-code`       | Código de salida de `pipelineguard`: `0` OK · `1` violación de seguridad real · `2` fallo de la herramienta. **El step no falla por `1` ni `2`** — ver `docs/action-run.md`. |
| `report-path` | `steps.run.outputs.report-path`     | Ruta del reporte Markdown generado (`pipelineguard-report.md`). |

El step 7 (`Enforce PipelineGuard result`, `action/scripts/gate.sh`) lee
`exit-code` y decide si el job completo falla — ver `docs/action-gate.md`.

## Qué versión del binario se instala

El step 1 calcula `PIPELINEGUARD_VERSION: ${{ inputs.version || github.action_ref }}`:
si el input `version` viene vacío (el default), usa `github.action_ref`, el ref
con el que el workflow llamante fijó la Action. La documentación oficial de
contextos dice que, en composite actions, `github.action_ref` hay que leerlo
desde `env:` (no dentro de `run:`); así se hace aquí. `install.sh`
(`resolve_version`, ver `docs/action-install.md`) decide qué hacer con el valor:

| Valor que recibe `install.sh` | Ejemplo de `uses:` | Binario instalado |
|---|---|---|
| `vX.Y.Z` o `vX.Y.Z-<pre>` | `…/action@v1.1.0` | Exactamente ese tag, sin consultar la API |
| `vN` | `…/action@v1` | El release `vN.X.Y` limpio más alto (API de Releases) |
| Otra cosa (vacío, rama, SHA) | `…/action@main`, `…/action@<sha>` | **Ninguno**: falla con `cannot infer the binary version from '<valor>'; pass the action's \`version:\` input (e.g. 'v1' or 'v1.1.0')` |

Con `uses: ./action` (una Action local, como en `self-scan.yml`) no hay un ref de
release del que inferir la versión, por eso ese workflow pasa `version: v1`.

**Verificado en ejecuciones reales** (runner `ubuntu-24.04`): con `@v1`,
`install.sh` resolvió `v1` a `v1.1.0`, descargó
`pipelineguard_1.1.0_linux_amd64.tar.gz` y verificó el checksum; con la Action
fijada a un tag completo (`@v1.1.0-rc.1`) y sin input `version:`, instaló el
binario de ese tag (`github.action_ref`).

> **Con `v1.0.x` el comportamiento es el anterior.** En esas versiones el
> default de `version` es `v1` y `github.action_ref` no se usa: fijar `@v1.0.0`
> sigue instalando el último `1.x`. Para fijar el binario en esas versiones hay
> que pasar `version:`.

## Steps

```yaml
runs:
  using: composite
  steps:
    - name: Install pipelineguard
      shell: bash
      env:
        PIPELINEGUARD_VERSION: ${{ inputs.version || github.action_ref }}
        GITHUB_TOKEN: ${{ github.token }}
      run: bash "${GITHUB_ACTION_PATH}/scripts/install.sh" "${PIPELINEGUARD_VERSION}"

    - name: Run pipelineguard
      id: run
      shell: bash
      env:
        CONFIG_PATH: ${{ inputs.config-path }}
        SARIF_OUTPUT: ${{ inputs.sarif-output }}
      run: bash "${GITHUB_ACTION_PATH}/scripts/run.sh"

    - name: Upload SARIF results
      id: sarif
      if: always()
      continue-on-error: true
      uses: github/codeql-action/upload-sarif@v4
      with:
        sarif_file: ${{ inputs.sarif-output }}

    - name: Post or update PR comment
      id: comment
      if: always() && github.event_name == 'pull_request'
      continue-on-error: true
      shell: bash
      env:
        REPORT_PATH: ${{ steps.run.outputs.report-path }}
        PR_NUMBER: ${{ github.event.pull_request.number }}
        GH_TOKEN: ${{ github.token }}
      run: bash "${GITHUB_ACTION_PATH}/scripts/comment.sh"

    - name: Add report to job summary
      if: always()
      shell: bash
      env:
        REPORT_PATH: ${{ steps.run.outputs.report-path }}
      run: |
        if [[ -n "${REPORT_PATH:-}" && -f "$REPORT_PATH" ]]; then
          cat "$REPORT_PATH" >>"$GITHUB_STEP_SUMMARY"
        fi

    - name: Warn about skipped integrations
      if: always() && (steps.sarif.outcome == 'failure' || steps.comment.outcome == 'failure')
      shell: bash
      env:
        SARIF_OUTCOME: ${{ steps.sarif.outcome }}
        COMMENT_OUTCOME: ${{ steps.comment.outcome }}
      run: |
        # one ::warning title=PipelineGuard::… per failed integration
        # (full text in action/action.yml)

    - name: Enforce PipelineGuard result
      if: always()
      shell: bash
      env:
        EXIT_CODE: ${{ steps.run.outputs.exit-code }}
      run: bash "${GITHUB_ACTION_PATH}/scripts/gate.sh"
```

1. **Install pipelineguard** — corre `scripts/install.sh` con
   `inputs.version || github.action_ref` como argumento posicional (ver "Qué
   versión del binario se instala"). `install.sh` usa tal cual una versión
   completa, resuelve un major desnudo (`v1`) a la release real más alta de esa
   serie (`resolve_version`, que consulta la API de Releases autenticándose con el
   `GITHUB_TOKEN` que este step le pasa) y rechaza cualquier otro valor. Luego detecta OS/arch, descarga el archive y `checksums.txt` desde
   GitHub Releases, **verifica el checksum**, extrae el binario y lo añade a
   `$GITHUB_PATH` (ver `docs/action-install.md`).
2. **Run pipelineguard** (`id: run`) — corre `scripts/run.sh`, que ejecuta el
   binario, manda su Markdown a `pipelineguard-report.md`, y escribe
   `exit-code` / `report-path` en `$GITHUB_OUTPUT`. **Nunca falla el step** por un
   exit code `1`/`2` del binario; sí falla si el binario no está en PATH.
3. **Upload SARIF results** (`id: sarif`) — sube el archivo SARIF a GitHub con
   la action oficial `github/codeql-action/upload-sarif`. `if: always()` para que
   corra aunque un step anterior haya fallado de verdad. `continue-on-error: true`:
   es una integración **opcional**; si falla, el step queda con
   `outcome: failure` pero no pone el job en rojo. Ver la sección siguiente.
4. **Post or update PR comment** (`id: comment`) — corre `scripts/comment.sh`,
   que publica el reporte Markdown como comentario del PR, **actualizando** el
   comentario anterior de PipelineGuard en vez de crear uno nuevo. Restringido a
   eventos `pull_request`. También `continue-on-error: true`. Ver
   `docs/action-comment.md`.
5. **Add report to job summary** — `if: always()`. Si
   `REPORT_PATH` (`steps.run.outputs.report-path`, vía `env:`) no está vacío y el
   archivo existe, hace `cat "$REPORT_PATH" >>"$GITHUB_STEP_SUMMARY"`: el reporte
   queda visible en la página del run aunque el comentario no se haya podido
   publicar (o en un `push`, donde no hay PR). Si no hay reporte (binario
   ausente), no hace nada y no falla.
6. **Warn about skipped integrations** — corre solo si
   `steps.sarif.outcome == 'failure' || steps.comment.outcome == 'failure'`
   (`outcome` es el resultado **antes** de aplicar `continue-on-error`; la
   `conclusion` de esos steps sería `success`). Emite un
   `::warning title=PipelineGuard::…` por cada integración fallida, con la causa
   probable: token de solo lectura en PRs desde forks, repo privado sin GitHub
   Advanced Security (SARIF) o falta de `pull-requests: write` (comentario). Los
   outcomes llegan por `env:` (`SARIF_OUTCOME`, `COMMENT_OUTCOME`), nunca
   interpolados en el `run:`. Un step que no corrió (p. ej. el comentario en un
   `push`) tiene `outcome: skipped` y no dispara el aviso.
7. **Enforce PipelineGuard result** — corre `scripts/gate.sh`, que lee
   `steps.run.outputs.exit-code` y es quien finalmente hace fallar (o no) el job:
   silencio en `0`, `exit 1` en `1` (violación de seguridad real), `exit 2` en `2`
   (fallo de la herramienta), y `exit 1` con mensaje de bug de wiring si el valor
   viene vacío o no es uno de esos tres códigos. `if: always()` para que corra
   siempre. Es el **último** step y el **único** que decide el resultado final del
   job: un fallo de los steps 3 o 4 no lo decide (en `v1.0.x` sí). Ver
   `docs/action-gate.md`.

## Step 3 — Subida de SARIF a GitHub Security

```yaml
- name: Upload SARIF results
  id: sarif
  if: always()
  continue-on-error: true
  uses: github/codeql-action/upload-sarif@v4
  with:
    sarif_file: ${{ inputs.sarif-output }}
```

### Por qué aquí `${{ }}` sí es apropiado

`sarif_file` es un **input declarativo de otra action**, no un valor que se
interpola dentro de un `run:` de bash. No hay superficie de inyección de shell en
este contexto, así que el patrón `env:` no aporta nada. (Los otros **seis**
steps, 1, 2 y 4 a 7, ejecutan bash y **todos** usan `env:`.)

### Qué pasa en GitHub cuando el SARIF se sube

- GitHub ingiere el archivo como resultado de **code scanning**. Cada `result` del
  SARIF (ver `docs/sarif.md`) aparece como una **alerta en la pestaña *Security* →
  *Code scanning*** del repo, con su nivel (`error`/`warning`/`note`), su regla,
  su ubicación (`archivo:línea`) y el `originTool` en las propiedades.
- En un Pull Request, las alertas nuevas introducidas por el PR se muestran
  además **inline en el diff** y como *check* del PR.
- Las alertas se **deduplican y versionan** por fingerprint entre corridas: si un
  hallazgo desaparece en una corrida posterior, GitHub lo marca como *fixed*.
- El *tool name* que se ve en la UI es `PipelineGuard` (viene de
  `runs[].tool.driver.name` en el SARIF).

### Qué pasa si la subida falla

Investigado en la documentación de `github/codeql-action/upload-sarif`:

- **Archivo inexistente** en la ruta de `sarif_file` → el step **falla** con un
  error tipo `Path does not exist: <ruta>`.
- **Archivo vacío o SARIF inválido** (no es JSON, o no cumple el schema 2.1.0) →
  el step **falla** en la validación (`Invalid SARIF`, error de parseo JSON).
- **Sin permiso o sin code scanning**: un PR desde un fork recibe un
  `GITHUB_TOKEN` de solo lectura (sin `security-events: write`), y un repo
  privado sin GitHub Advanced Security no tiene code scanning: en ambos casos la
  subida puede fallar.
- En el **camino normal** el archivo no falla la validación: el binario
  `pipelineguard` siempre escribe un SARIF **válido** aunque no haya hallazgos
  (`"results": []`, `"rules": []`, nunca `null` — requisito de `docs/sarif.md`).

Por `continue-on-error: true`, cualquiera de esos fallos deja el
step con `outcome: failure` pero **no** pone el job en rojo: el step 6 emite un
`::warning` y el reporte queda en el resumen del job (step 5). En `v1.0.x`, el
fallo de este step hacía fallar el job aunque el escaneo hubiera ido bien.

### Versión pineada `@v4`

El step usa `github/codeql-action/upload-sarif@v4`, el major vigente (antes
estaba en `@v3`; se actualizó antes de cortar v1.0.0). Pinear al tag mayor
flotante sigue la misma convención que el resto del proyecto. Desde el Bloque 23,
Dependabot revisa este `action.yml` cada semana (`.github/dependabot.yml`,
entrada `github-actions` con `directory: /action`).

## Si falta el binario

Si no hay binario `pipelineguard`, hay dos variantes:

- **`install.sh` falla** (release no encontrada, versión no inferible, checksum
  inválido, descarga rota…): *Install* queda en rojo. *Run pipelineguard* no
  tiene `if:`, así que GitHub lo **omite** (skipped), no lo marca como fallido.
- **La instalación "termina" pero el binario no está en PATH**: *Run
  pipelineguard* falla (`run.sh` sale con `exit 1`: `pipelineguard is not on
  PATH — the install step must run first`).

En ambos casos, después (comportamiento de `v1.1.0`):

| Step | Resultado | Por qué |
|------|-----------|---------|
| 3. Upload SARIF | `outcome: failure`, no pone el job en rojo | `if: always()`; no existe el archivo SARIF; `continue-on-error: true`. |
| 4. PR comment (solo en `pull_request`) | `outcome: failure`, no pone el job en rojo | `if: always()`; `report-path` viene vacío, `comment.sh` cae al default `pipelineguard-report.md`, que no existe → `report file … not found`; `continue-on-error: true`. |
| 5. Job summary | ✅ no hace nada | `REPORT_PATH` vacío o el archivo no existe. |
| 6. Warn | ⚠️ emite el/los `::warning` | Porque 3 (y 4) fallaron. La "causa probable" que menciona (forks, GHAS) **no** es la real en este caso. |
| 7. Enforce | ❌ falla (`exit 1`) | `if: always()`; `EXIT_CODE` vacío → mensaje de **bug de wiring de la Action** (ver `docs/action-gate.md`). |

El job termina en rojo (por el gate), que es lo correcto. Ni el aviso del step 6
ni el mensaje de *Enforce* dan la causa real: está en el primer step en rojo
(*Install* o *Run*). En `v1.0.x`, los steps 3 y 4 también quedaban en rojo.

### Detalles de implementación

- **`env:` en vez de interpolar `${{ }}` dentro de `run:`** — los seis steps
  que ejecutan bash (1, 2 y 4 a 7) exponen sus valores como variables de entorno
  y los scripts las leen desde ahí. Evita el riesgo de inyección de shell de
  meter expresiones `${{ }}` directamente en el cuerpo de `run:`. Eso incluye
  `github.action_ref` (step 1) y los `steps.<id>.outcome` (step 6).
- **`${GITHUB_ACTION_PATH}`** — GitHub lo setea a la carpeta de la action
  (`action/`), así que `${GITHUB_ACTION_PATH}/scripts/…` resuelve a los scripts sin
  importar desde qué repo se invoque la action.
- **`shell: bash`** — obligatorio en composite actions; garantiza bash en Linux,
  macOS y Windows runners.
- **`if: always()` en los steps 3 a 7** — corren aunque un step anterior falle
  (el 6, además, solo si falló una integración). El step 4 añade
  `&& github.event_name == 'pull_request'` porque solo hay un PR al que comentar
  en ese tipo de evento (ver `docs/action-comment.md`). El step 7 (`gate.sh`) es
  el que finalmente hace fallar el job por el resultado del análisis de
  seguridad.
- **`continue-on-error: true` en los steps 3 y 4** — son
  integraciones opcionales: su fallo se reporta (step 6) pero no decide el
  resultado del job.
- **`GH_TOKEN: ${{ github.token }}`** — el step 4 usa `gh api`, que lee el token
  de `GH_TOKEN`. Es el `GITHUB_TOKEN` del job; necesita `pull-requests: write`.
- **`GITHUB_TOKEN: ${{ github.token }}`** en el step 1 — `install.sh` lo usa solo
  para autenticar la consulta a la API de Releases con la que resuelve `v1` a una
  versión exacta. Así evita el límite anónimo de 60 req/h por IP. Solo lee
  releases públicas y no necesita permisos extra.
- `branding` (`icon: shield`, `color: purple`) — solo estética para el
  Marketplace.

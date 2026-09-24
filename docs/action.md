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
> `docs/action-comment.md`).

**Versión completa.** Con el Bloque 16 (`gate.sh`) `action.yml` llega a su forma
final: los **5 steps** descritos abajo son todos los que tiene la composite
action. No queda ningún step pendiente por añadir en un bloque futuro.

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
| `version`      | `v1`                          | Tag de release del binario a instalar (`v1`, `v1.2.3`, …). Se pasa a `install.sh`. |
| `config-path`  | `.pipelineguard.yml`          | Ruta al archivo de configuración. Se pasa a `run.sh` como `CONFIG_PATH`. |
| `sarif-output` | `pipelineguard-results.sarif` | Ruta del reporte SARIF. Se pasa a `run.sh` como `SARIF_OUTPUT`. |

## Outputs

| Output        | Origen                              | Descripción |
|---------------|-------------------------------------|-------------|
| `exit-code`   | `steps.run.outputs.exit-code`       | Código de salida de `pipelineguard`: `0` OK · `1` violación de seguridad real · `2` fallo de la herramienta. **El step no falla por `1` ni `2`** — ver `docs/action-run.md`. |
| `report-path` | `steps.run.outputs.report-path`     | Ruta del reporte Markdown generado (`pipelineguard-report.md`). |

El step 5 (`Enforce PipelineGuard result`, `action/scripts/gate.sh`) lee
`exit-code` y decide si el job completo falla — ver `docs/action-gate.md`.

## Steps

```yaml
runs:
  using: composite
  steps:
    - name: Install pipelineguard
      shell: bash
      env:
        PIPELINEGUARD_VERSION: ${{ inputs.version }}
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
      if: always()
      uses: github/codeql-action/upload-sarif@v4
      with:
        sarif_file: ${{ inputs.sarif-output }}

    - name: Post or update PR comment
      if: always() && github.event_name == 'pull_request'
      shell: bash
      env:
        REPORT_PATH: ${{ steps.run.outputs.report-path }}
        PR_NUMBER: ${{ github.event.pull_request.number }}
        GH_TOKEN: ${{ github.token }}
      run: bash "${GITHUB_ACTION_PATH}/scripts/comment.sh"

    - name: Enforce PipelineGuard result
      if: always()
      shell: bash
      env:
        EXIT_CODE: ${{ steps.run.outputs.exit-code }}
      run: bash "${GITHUB_ACTION_PATH}/scripts/gate.sh"
```

1. **Install pipelineguard** — corre `scripts/install.sh` con el input `version`
   como argumento posicional. `install.sh` resuelve un major desnudo (`v1`, el
   default) a la release real más alta de esa serie (`resolve_version`, que
   consulta la API de Releases autenticándose con el `GITHUB_TOKEN` que este step
   le pasa). Luego detecta OS/arch, descarga el archive y `checksums.txt` desde
   GitHub Releases, **verifica el checksum**, extrae el binario y lo añade a
   `$GITHUB_PATH` (ver `docs/action-install.md`).
2. **Run pipelineguard** (`id: run`) — corre `scripts/run.sh`, que ejecuta el
   binario, manda su Markdown a `pipelineguard-report.md`, y escribe
   `exit-code` / `report-path` en `$GITHUB_OUTPUT`. **Nunca falla el step** por un
   exit code `1`/`2` del binario; sí falla si el binario no está en PATH.
3. **Upload SARIF results** — sube el archivo SARIF a GitHub con la action oficial
   `github/codeql-action/upload-sarif`. `if: always()` para que corra aunque un
   step anterior haya fallado de verdad. Ver la sección siguiente.
4. **Post or update PR comment** — corre `scripts/comment.sh`, que publica el
   reporte Markdown como comentario del PR, **actualizando** el comentario
   anterior de PipelineGuard en vez de crear uno nuevo. Restringido a eventos
   `pull_request`. Ver `docs/action-comment.md`.
5. **Enforce PipelineGuard result** — corre `scripts/gate.sh`, que lee
   `steps.run.outputs.exit-code` y es quien finalmente hace fallar (o no) el job:
   silencio en `0`, `exit 1` en `1` (violación de seguridad real), `exit 2` en `2`
   (fallo de la herramienta), y `exit 1` con mensaje de bug de wiring si el valor
   viene vacío o no es uno de esos tres códigos. `if: always()` para que corra
   incluso si los steps 3 o 4 fallaron de verdad. Es el **único** step de la
   Action al que le corresponde decidir el resultado final del job en base al
   análisis de seguridad. Ver `docs/action-gate.md`.

## Step 3 — Subida de SARIF a GitHub Security

```yaml
- name: Upload SARIF results
  if: always()
  uses: github/codeql-action/upload-sarif@v4
  with:
    sarif_file: ${{ inputs.sarif-output }}
```

### Por qué aquí `${{ }}` sí es apropiado

`sarif_file` es un **input declarativo de otra action**, no un valor que se
interpola dentro de un `run:` de bash. No hay superficie de inyección de shell en
este contexto, así que el patrón `env:` no aporta nada. (Los otros **cuatro**
steps, 1, 2, 4 y 5, ejecutan bash y **todos** usan `env:`.)

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

### Qué pasa si el archivo no existe o está vacío

Investigado en la documentación de `github/codeql-action/upload-sarif`:

- **Archivo inexistente** en la ruta de `sarif_file` → el step **falla** con un
  error tipo `Path does not exist: <ruta>`. No se traga el error silenciosamente.
- **Archivo vacío o SARIF inválido** (no es JSON, o no cumple el schema 2.1.0) →
  el step **falla** en la validación (`Invalid SARIF`, error de parseo JSON).
- Como el step lleva `if: always()`, corre incluso si no hubo binario (en ese
  caso **no** se escribió ningún SARIF) y **también** falla por "archivo
  inexistente". Ver "Si falta el binario: fallo en cascada" abajo.
- En el **camino normal** esto no ocurre: el binario `pipelineguard` siempre
  escribe un SARIF **válido** aunque no haya hallazgos (`"results": []`,
  `"rules": []`, nunca `null` — requisito de `docs/sarif.md`), así que el archivo
  siempre existe y siempre pasa la validación.

### Versión pineada `@v4`

El step usa `github/codeql-action/upload-sarif@v4`, el major vigente (antes
estaba en `@v3`; se actualizó antes de cortar v1.0.0). Pinear al tag mayor
flotante sigue la misma convención que el resto del proyecto.

## Si falta el binario: fallo en cascada

Si no hay binario `pipelineguard`, **no** falla un solo step. Por los
`if: always()` de los steps 3, 4 y 5, fallan varios en cascada. Hay dos variantes:

- **`install.sh` falla** (release no encontrada, checksum inválido, descarga
  rota…): *Install* queda en rojo. *Run pipelineguard* no tiene `if:`, así que
  GitHub lo **omite** (skipped), no lo marca como fallido.
- **La instalación "termina" pero el binario no está en PATH**: *Run
  pipelineguard* falla (`run.sh` sale con `exit 1`: `pipelineguard is not on
  PATH — the install step must run first`).

En ambos casos, después:

| Step | Resultado | Por qué |
|------|-----------|---------|
| 3. Upload SARIF | ❌ falla | `if: always()`; no existe el archivo SARIF. |
| 4. PR comment (solo en `pull_request`) | ❌ falla | `if: always()`; `steps.run.outputs.report-path` viene vacío, `comment.sh` cae al default `pipelineguard-report.md`, que no existe → `report file … not found`. |
| 5. Enforce | ❌ falla (`exit 1`) | `if: always()`; `EXIT_CODE` vacío → mensaje de **bug de wiring de la Action** (ver `docs/action-gate.md`). |

El mensaje de *Enforce* habla de un bug de wiring aunque la causa real sea la
instalación. El primer step en rojo (*Install* o *Run*) es el que tiene la causa
real.

### Detalles de implementación

- **`env:` en vez de interpolar `${{ }}` dentro de `run:`** — los cuatro steps
  que ejecutan bash (1, 2, 4 y 5) exponen sus valores como variables de entorno
  y los scripts las leen desde ahí. Evita el riesgo de inyección de shell de
  meter expresiones `${{ }}` directamente en el cuerpo de `run:`.
- **`${GITHUB_ACTION_PATH}`** — GitHub lo setea a la carpeta de la action
  (`action/`), así que `${GITHUB_ACTION_PATH}/scripts/…` resuelve a los scripts sin
  importar desde qué repo se invoque la action.
- **`shell: bash`** — obligatorio en composite actions; garantiza bash en Linux,
  macOS y Windows runners.
- **`if: always()` en los steps 3, 4 y 5** — corren aunque un step anterior
  falle. En el camino normal `run.sh` termina con `exit 0` y no haría falta, pero
  se agrega por robustez. El step 4 añade `&& github.event_name == 'pull_request'`
  porque solo hay un PR al que comentar en ese tipo de evento (ver
  `docs/action-comment.md`). El step 5 (`gate.sh`) es el que finalmente hace
  fallar el job por el resultado del análisis de seguridad — necesita `always()`
  precisamente para poder correr y emitir ese veredicto incluso si los steps 3 o 4
  fallaron por su cuenta (ver `docs/action-gate.md`).
- **`GH_TOKEN: ${{ github.token }}`** — el step 4 usa `gh api`, que lee el token
  de `GH_TOKEN`. Es el `GITHUB_TOKEN` del job; necesita `pull-requests: write`.
- **`GITHUB_TOKEN: ${{ github.token }}`** en el step 1 — `install.sh` lo usa solo
  para autenticar la consulta a la API de Releases con la que resuelve `v1` a una
  versión exacta. Así evita el límite anónimo de 60 req/h por IP. Solo lee
  releases públicas y no necesita permisos extra.
- `branding` (`icon: shield`, `color: purple`) — solo estética para el
  Marketplace.

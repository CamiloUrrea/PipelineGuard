# Composite action (`action/action.yml`)

`action/action.yml` es la GitHub composite action que envuelve todo PipelineGuard:
instala el binario, lo ejecuta, y expone el resultado como outputs. Un repo la usa
con una línea:

```yaml
- uses: CamiloUrrea/PipelineGuard/action@v1
```

> ⚠️ **Sin tests automáticos.** `action.yml` en sí **no** tiene pruebas: solo se
> puede validar corriendo la Action de verdad en un workflow real, y eso
> **todavía no se ha hecho** (no hay release publicada ni workflow de ejemplo).
> Lo único verificado localmente es que el archivo es YAML sintácticamente válido
> y que sus dos scripts (`install.sh`, `run.sh`) pasan shellcheck + bats por
> separado.

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

Un bloque futuro añadirá el step que lee `exit-code` y decide si el job completo
falla.

## Steps

```yaml
runs:
  using: composite
  steps:
    - name: Install pipelineguard
      shell: bash
      env:
        PIPELINEGUARD_VERSION: ${{ inputs.version }}
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
      uses: github/codeql-action/upload-sarif@v3
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
```

1. **Install pipelineguard** — corre `scripts/install.sh` con el tag de release
   como argumento posicional. `install.sh` detecta OS/arch, descarga el archive y
   `checksums.txt` desde GitHub Releases, **verifica el checksum**, extrae el
   binario y lo añade a `$GITHUB_PATH` (ver `docs/action-install.md`).
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

## Step 3 — Subida de SARIF a GitHub Security

```yaml
- name: Upload SARIF results
  if: always()
  uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: ${{ inputs.sarif-output }}
```

### Por qué aquí `${{ }}` sí es apropiado

`sarif_file` es un **input declarativo de otra action**, no un valor que se
interpola dentro de un `run:` de bash. No hay superficie de inyección de shell en
este contexto, así que el patrón `env:` no aporta nada. (En los steps 1 y 2, que
sí ejecutan bash, se sigue usando `env:`.)

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
- Como el step lleva `if: always()`, corre incluso si el step `run` terminó con
  error real (binario ausente, en cuyo caso **no** se escribió ningún SARIF). En
  ese caso este step **también** fallará por "archivo inexistente" — el resultado
  neto es un job con dos steps en rojo, lo cual es correcto: la corrida ya estaba
  rota.
- En el **camino normal** esto no ocurre: el binario `pipelineguard` siempre
  escribe un SARIF **válido** aunque no haya hallazgos (`"results": []`,
  `"rules": []`, nunca `null` — requisito de `docs/sarif.md`), así que el archivo
  siempre existe y siempre pasa la validación.

### Nota sobre la versión pineada `@v3`

`github/codeql-action` va por el major **v3** (v3 salió en enero de 2024 y
reemplazó a v2, que dejó de recibir soporte). Hasta donde llega mi conocimiento
(**corte: enero de 2026**) **v3 es el major vigente y no existe un v4**. Si desde
entonces se publicó un major más nuevo, convendría revisarlo; `@v3` sigue la
convención de pinear al tag mayor flotante, igual que el resto del proyecto.

### Detalles de implementación

- **`env:` en vez de interpolar `${{ }}` dentro de `run:`** — los valores de los
  inputs se exponen como variables de entorno y los scripts las leen desde ahí.
  Evita el riesgo de inyección de shell de meter expresiones `${{ }}` directamente
  en el cuerpo de `run:`.
- **`${GITHUB_ACTION_PATH}`** — GitHub lo setea a la carpeta de la action
  (`action/`), así que `${GITHUB_ACTION_PATH}/scripts/…` resuelve a los scripts sin
  importar desde qué repo se invoque la action.
- **`shell: bash`** — obligatorio en composite actions; garantiza bash en Linux,
  macOS y Windows runners.
- **`if: always()` en los steps 3 y 4** — corren aunque un step anterior falle.
  En el camino normal `run.sh` termina con `exit 0` y no haría falta, pero se
  agrega por robustez. El step 4 añade `&& github.event_name == 'pull_request'`
  porque solo hay un PR al que comentar en ese tipo de evento (ver
  `docs/action-comment.md`).
- **`GH_TOKEN: ${{ github.token }}`** — el step 4 usa `gh api`, que lee el token
  de `GH_TOKEN`. Es el `GITHUB_TOKEN` del job; necesita `pull-requests: write`.
- `branding` (`icon: shield`, `color: purple`) — solo estética para el
  Marketplace.

# Ejecución del binario (`action/scripts/run.sh`)

`action/scripts/run.sh` ejecuta el binario `pipelineguard` (instalado antes por
`install.sh`, ver `docs/action-install.md`) y **expone su código de salida como
output del step** en vez de hacer fallar el step. Lo consume la composite action
(`action.yml`).

## Funciones

- `require_cmd <name>` — copia local, **duplicada a propósito** de `install.sh`
  (no se comparte código entre scripts). Falla ruidoso si el comando no está.
- `run_pipelineguard <config_path> <sarif_output> <report_path>`:
  1. Ejecuta `pipelineguard --config "$config_path" --sarif-output "$sarif_output"`.
  2. Redirige su **stdout** (el reporte Markdown) a `report_path`.
  3. Captura su exit code con `|| exit_code=$?`, de modo que `set -e` **no**
     termine el script.
  4. Hace `echo "$exit_code"` — ese número es el "valor de retorno" que el flujo
     principal captura por command substitution.

## Flujo principal (detrás del guard `BASH_SOURCE`)

1. `require_cmd pipelineguard` — **si falla, `exit 1` inmediato y ruidoso**
   (ver "Por qué" abajo).
2. Lee `CONFIG_PATH` y `SARIF_OUTPUT` del entorno, con defaults
   `.pipelineguard.yml` y `pipelineguard-results.sarif`.
3. Llama a `run_pipelineguard`, guarda el exit code capturado.
4. Escribe en `$GITHUB_OUTPUT` (si está definido):
   - `exit-code=<código>`
   - `report-path=pipelineguard-report.md`
5. **`exit 0` siempre** (salvo el caso 1). El resultado real del análisis vive en
   el output `exit-code`, no en el exit status del script.

El guard `if [[ "${BASH_SOURCE[0]}" == "${0}" ]]` permite que bats sourcee el
script para probar las funciones sin disparar el flujo.

## Por qué el script NO falla ante un exit code 1 o 2 de `pipelineguard`

`pipelineguard` puede salir con (ver `docs/cmd.md`):

| Código | Significado |
|--------|-------------|
| `0` | OK, nada supera el umbral. |
| `1` | Violación de seguridad real (según `enforce` / `fail_threshold`). |
| `2` | Fallo de la herramienta (config inválido, error de escáner, error de E/S). |

Si `run.sh` hiciera fallar el step inmediatamente al ver `1` o `2`, los steps
**posteriores** de la Action —subir el SARIF a la pestaña Security, publicar el
comentario del PR— **no se ejecutarían** (un step fallido corta el job salvo
`if: always()`, y aún así el estado queda contaminado). Y esos pasos deben correr
**siempre**: el valor de PipelineGuard es el reporte, lo encuentre o no.

Por eso:

- `run.sh` guarda el código en el output `exit-code` y termina con `exit 0`.
- Un **step futuro** (bloque posterior), después de subir SARIF y comentar el PR,
  leerá `steps.run.outputs.exit-code` y **ahí** decidirá si el job completo falla
  (típicamente: fallar si es `1`; y `2` según política del equipo).

Esto separa limpiamente "hubo un hallazgo" de "el pipeline se rompió", y deja la
decisión de bloqueo en un único lugar explícito.

## La excepción: binario ausente

Si `pipelineguard` **no está en PATH**, `install.sh` (Bloque 12) falló en su
trabajo. No hay nada que diferir, ningún análisis que reportar, ningún exit code
que guardar. En ese caso `run.sh` **sí** termina de inmediato con código ≠ 0 y un
mensaje claro (`pipelineguard is not on PATH — the install step must run first`) —
es un fallo duro de infraestructura de la Action, no del análisis de seguridad.

## Correr shellcheck y bats

```sh
shellcheck action/scripts/run.sh
bats action/scripts/run_test.bats
```

`run_test.bats` pone un `pipelineguard` **falso** primero en PATH (un stub que
imprime contenido conocido y sale con el código que el test necesita — nunca el
binario real) y verifica:

- Stub sale `0` → `$GITHUB_OUTPUT` tiene `exit-code=0`, el reporte contiene el
  stdout del stub, y `run.sh` termina con `exit 0`.
- Stub sale `1` → `exit-code=1` capturado, pero `run.sh` **igual** termina con
  `exit 0`.
- Stub sale `2` → `exit-code=2` capturado, `run.sh` termina con `exit 0`.
- `CONFIG_PATH` / `SARIF_OUTPUT` se pasan como `--config` / `--sarif-output` al
  binario.
- `pipelineguard` ausente de PATH → `run.sh` termina con código ≠ 0 y mensaje
  claro.

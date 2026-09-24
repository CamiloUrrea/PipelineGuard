# Enforcement final (`action/scripts/gate.sh`)

`action/scripts/gate.sh` es el **quinto y último step** de la composite action
(`action/action.yml`). Es el único lugar donde el resultado de `pipelineguard`
puede hacer fallar el job completo — todos los steps anteriores (instalar, correr,
subir SARIF, comentar el PR) corren siempre, sin importar el veredicto (ver
`docs/action-run.md` y `docs/action.md`).

Lee `EXIT_CODE`, que la Action puebla desde `steps.run.outputs.exit-code` (el
output que `run.sh` escribió en el Bloque 13), y decide qué hacer.

## Los 5 casos

| `EXIT_CODE`              | Resultado          | Mensaje (stderr)                                                    |
|---------------------------|--------------------|-----------------------------------------------------------------------|
| `"0"`                     | `exit 0`, silencio | (ninguno) — nada superó el umbral de política.                       |
| `"1"`                     | `exit 1`           | Violación de seguridad real detectada por PipelineGuard.             |
| `"2"`                     | `exit 2`           | Fallo interno de la herramienta — **no** es un veredicto de seguridad. |
| vacío / sin setear        | `exit 1`           | Bug de wiring de la propia Action, no un veredicto de pipelineguard.  |
| no numérico o numérico fuera de `{0,1,2}` (ej. `"abc"`, `"3"`) | `exit 1` | Mismo mensaje que el caso anterior. |

## Por qué el caso vacío/inválido se trata como fallo de la Action, no de seguridad

`run.sh` (Bloque 13) **siempre** escribe `exit-code=<0|1|2>` en `$GITHUB_OUTPUT`
cuando el binario existe. Es decir: **si los steps anteriores hicieron su
trabajo, `EXIT_CODE` siempre es `"0"`, `"1"` o `"2"`.**

### El caso "binario ausente" **sí** llega a `gate.sh`

Si falta el binario (porque `install.sh` falló, o porque `run.sh` abortó con
`pipelineguard is not on PATH`), **no** se escribe ningún `exit-code`. Aun así
`gate.sh` **sí se ejecuta**: el step tiene `if: always()`. Corre con
`EXIT_CODE` **vacío**, cae en la rama `*)` y sale con `exit 1` y el mensaje de
*bug de wiring de la Action*. El job termina en rojo, que es lo correcto, pero
la causa real está en el primer step fallido (*Install* o *Run*), no en el
wiring. Ver "Si falta el binario: fallo en cascada" en `docs/action.md`.

Fuera de ese caso de binario ausente, que `gate.sh` reciba algo distinto a esos
tres valores (vacío, `"abc"`, o incluso un número que no sea `0`/`1`/`2`) solo
puede significar que algo se rompió en el *wiring* de la Action misma:

- alguien cambió el `id` del step `run` y el `env:` de este step quedó apuntando a
  un output que ya no existe;
- una expresión `${{ }}` mal escrita en `action.yml`;
- un fork/versión de la Action que saltó el step `run` por error.

Ninguno de esos casos es "PipelineGuard encontró (o no) un problema de
seguridad" — son bugs de configuración. Tratarlos como `exit 1` con un mensaje que
dice explícitamente *"esto es un bug de la Action, no un veredicto de
pipelineguard"* evita que alguien lea un fallo de CI ambiguo y piense que hay una
vulnerabilidad real cuando en realidad la Action está mal cableada.

### Por qué nunca se hace `exit "$EXIT_CODE"` directamente

Si `EXIT_CODE` no es un entero válido (por ejemplo `"abc"`), pasarlo directo a
`exit` no produce un error claro — el comportamiento de bash en ese caso es
inconsistente/silencioso según la versión, en vez de fallar de forma explícita.
`gate.sh` evita esto por completo usando un `case "$exit_code" in 0|1|2|*)`: es un
match de *string*, nunca una expresión aritmética, así que un valor no numérico
simplemente cae en la rama `*)` sin que bash intente interpretarlo como número.

## El step en `action.yml`

```yaml
- name: Enforce PipelineGuard result
  if: always()
  shell: bash
  env:
    EXIT_CODE: ${{ steps.run.outputs.exit-code }}
  run: bash "${GITHUB_ACTION_PATH}/scripts/gate.sh"
```

- **`if: always()`** — corre aunque un step anterior (subida de SARIF, comentario
  del PR) haya fallado de verdad, para que el job siempre termine con un
  resultado explícito relacionado con el análisis de seguridad.
- **`env:` en vez de interpolar `${{ }}` en `run:`** — mismo patrón que el resto
  de la Action (ver `docs/action.md`): el valor pasa por una variable de entorno,
  no se pega directo en el cuerpo del script.

## Correr shellcheck y bats

```sh
shellcheck action/scripts/gate.sh
bats action/scripts/gate_test.bats
```

`gate_test.bats` no toca el binario `pipelineguard`, red ni GitHub — solo setea
`EXIT_CODE` y verifica el exit code y el contenido de stderr del script:

- `EXIT_CODE=0` → `exit 0`, sin ninguna salida.
- `EXIT_CODE=1` → `exit 1`, el mensaje menciona una violación de seguridad.
- `EXIT_CODE=2` → `exit 2`, el mensaje distingue un fallo interno de la
  herramienta (y explícitamente no menciona "violación de seguridad").
- `EXIT_CODE` sin setear → `exit 1`, el mensaje apunta a un bug de la Action.
- `EXIT_CODE=""` (vacío) → mismo tratamiento que el caso anterior.
- `EXIT_CODE="abc"` (no numérico) → mismo tratamiento, y se confirma que nunca se
  intenta un `exit "abc"` directo.
- `EXIT_CODE="3"` (numérico pero no es uno de los tres códigos legítimos) →
  mismo tratamiento que el caso de wiring roto.

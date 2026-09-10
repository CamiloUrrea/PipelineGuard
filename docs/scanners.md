# Ejecución real de escáneres (`internal/scanners`)

El paquete `internal/scanners` contiene las implementaciones **reales** de
`orchestrator.ScannerFunc` (`func() ([]byte, error)`):

```go
func RunGitleaks() ([]byte, error)
func RunTrivy()    ([]byte, error)
```

Localizan el binario en `PATH`, lo ejecutan contra el directorio actual y
devuelven los bytes crudos del reporte JSON, listos para `parsers.ParseGitleaks`
/ `parsers.ParseTrivy`.

## Patrón: archivo temporal para el reporte

Cada función:

1. **Verifica que el binario exista** con `exec.LookPath`. Si no está, devuelve
   un error accionable, no un error crudo de Go:
   `"gitleaks no encontrado en PATH: instálalo desde https://github.com/gitleaks/gitleaks"`.
2. **Reserva un archivo temporal** con `os.CreateTemp` (`pipelineguard-<tool>-*.json`)
   y cierra el descriptor: el escáner lo abrirá por nombre.
   `defer os.Remove(...)` garantiza que se borra pase lo que pase.
3. **Ejecuta el binario** apuntando su reporte a ese archivo
   (`--report-path` en gitleaks, `--output` en trivy), y captura `stderr` para
   incluirlo en el mensaje de error si algo falla.
4. Lee el archivo temporal y, si no hubo fallo real, devuelve sus bytes.

Escribir a un archivo es **más confiable que capturar `stdout`**: los escáneres
mezclan líneas de log (progreso, warnings de deprecación, banner) con la salida,
y eso rompería el parseo JSON.

### Comandos usados

| Escáner  | Comando                                                              |
|----------|--------------------------------------------------------------------|
| gitleaks | `gitleaks detect --report-format json --report-path <tmp> --no-banner` |
| trivy    | `trivy fs --format json --output <tmp> .`                           |

(Coinciden con lo documentado en `docs/parsers.md`.)

## La trampa de los códigos de salida: `isExecutionFailure`

```go
func isExecutionFailure(runErr error, reportFileHasContent bool) bool
```

Función **pura y testeable por separado**. Separa dos cosas que se confunden
fácil:

- **"El proceso no se pudo ejecutar en absoluto"** → error real. Binario no
  encontrado, sin permisos de ejecución, fallo al montar el comando, timeout,
  error de E/S.
- **"El proceso corrió y salió con código ≠ 0"** → **NO es necesariamente un
  error.** Varias herramientas usan el código de salida para señalar "encontré
  hallazgos", no "me rompí".

`exit code != 0` **no** significa fallo de forma fiable:

| Herramienta | Sin hallazgos | Con hallazgos | Fallo real (args inválidos, binario roto) |
|-------------|---------------|---------------|-------------------------------------------|
| **gitleaks** | `0` | `1` (código de "leaks encontrados"; configurable con `--exit-code`) | `1` también → **ambiguo solo por el código** |
| **trivy** | `0` | `0` por defecto (necesita `--exit-code 1` para cambiarlo; no lo pasamos) | `> 0` |

> ⚠️ **Verificación en esta máquina:** ni `gitleaks` ni `trivy` están instalados
> aquí (`exec.LookPath` falla para ambos), así que **no pude confirmar los
> códigos de salida ejecutándolos**. La tabla de arriba viene de la
> documentación oficial de cada herramienta (gitleaks: flag `--exit-code`, default
> `1` para leaks; trivy: sección "Exit Codes", default `0` salvo `--exit-code`).
> `isExecutionFailure` está diseñada precisamente para **no depender** de que ese
> detalle sea exacto: se apoya en si el reporte se generó o no.

### Regla de decisión

`isExecutionFailure` devuelve `true` (fallo real) solo cuando:

1. `runErr` es `*exec.Error` → el proceso **nunca arrancó** (binario ausente, no
   ejecutable, permisos). Siempre fallo.
2. `runErr` es `*exec.ExitError` (corrió, salió ≠ 0) **y** el archivo de reporte
   quedó vacío o no se generó (`reportFileHasContent == false`). Señal real de
   fallo.
3. `runErr` es cualquier otro error no nulo (setup del comando, E/S, timeout).

Devuelve `false` (éxito) cuando:

- `runErr == nil`, o
- el proceso salió con código ≠ 0 **pero** el archivo de reporte tiene contenido
  → se trata como "encontró hallazgos", se lee el reporte y se devuelve.

## Qué SÍ y qué NO cubren los tests automáticos

**Cubierto** (`internal/scanners/scanners_test.go`, sin binarios instalados):

- `isExecutionFailure` como función pura: `runErr=nil`; `*exec.Error` (binario no
  encontrado) → `true` con y sin contenido; `*exec.ExitError` con
  `reportFileHasContent=true` → `false`; con `false` → `true`; otro error
  arbitrario → `true`.
- `RunGitleaks()` / `RunTrivy()` cuando el binario **no está en PATH**: el nombre
  del binario es una variable de paquete (`gitleaksBinary`, `trivyBinary`) que el
  test sobreescribe con un nombre inexistente. Se verifica que el error menciona
  el nombre de la herramienta, `PATH` y la URL de instalación.
- Chequeo en tiempo de compilación de que `RunGitleaks` y `RunTrivy` satisfacen
  `orchestrator.ScannerFunc`.

**NO cubierto en este bloque** (y así está anotado en el código):

- El **"camino feliz"**: un binario real de gitleaks/trivy ejecutándose y
  generando un reporte JSON válido. Requiere los binarios instalados en la
  máquina, se verifica **manualmente**, y todavía **no** corre en CI. Un test de
  integración end-to-end contra `pipelineguard-demo-vulnerable-app` (ver
  `ARCHITECTURE.md`, "Estrategia de testing") es trabajo de un bloque posterior.
- El comportamiento exacto de los códigos de salida de cada herramienta con
  hallazgos reales (no verificable sin los binarios; ver aviso de arriba).

## Cómo correr los tests

```sh
go test ./internal/scanners/...
```

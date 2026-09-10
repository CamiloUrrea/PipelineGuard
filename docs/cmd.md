# Binario CLI (`cmd/pipelineguard`)

`cmd/pipelineguard/main.go` es el ejecutable final: un **único comando** de Cobra
(sin subcomandos) que corre todo el flujo de PipelineGuard de principio a fin.

## Flujo

1. Carga la configuración (`config.Load`).
2. Corre el orquestador (`orchestrator.Run` con `scanners.RunGitleaks` /
   `scanners.RunTrivy` ya inyectados).
3. Escribe el reporte SARIF a disco (`--sarif-output`).
4. Imprime el reporte Markdown a **stdout**.
5. Sale con un código según el resultado (ver tabla abajo).

## Flags

| Flag             | Tipo   | Default                          | Descripción                                    |
|------------------|--------|----------------------------------|------------------------------------------------|
| `--config`       | string | `.pipelineguard.yml`             | Ruta al archivo de configuración. Es opcional: si no existe, se usan los defaults (ver `docs/config.md`). |
| `--sarif-output` | string | `pipelineguard-results.sarif`    | Ruta donde se escribe el reporte SARIF 2.1.0.  |

## Esquema de exit codes

Es la **interfaz del binario con CI**. Los tres casos son deliberadamente
distintos:

| Código | Significado                                                                                   |
|--------|----------------------------------------------------------------------------------------------|
| `0`    | Ejecución exitosa. Ningún hallazgo supera el umbral configurado (o `enforce: false`).         |
| `1`    | Ejecución exitosa, pero `policy.ShouldFail` determinó que el build debe fallar. **Es una violación de seguridad real, no un error de la herramienta.** |
| `2`    | **Fallo de la herramienta en sí**: config inválido, error de un escáner, error escribiendo el archivo SARIF. Nunca se confunde con el caso `1`. |

La distinción `1` vs `2` importa: un pipeline puede querer tratar "encontramos un
secreto" (`1`) distinto de "PipelineGuard se rompió" (`2`) — por ejemplo, marcar
el primero como fallo de seguridad y el segundo como fallo de infraestructura.

La función que decide el código es pura:

```go
func exitCode(err error, shouldFail bool) int
// err != nil            → 2   (un error de herramienta siempre gana)
// err == nil && shouldFail → 1
// resto                 → 0
```

## stdout vs stderr

- **stdout** → **solo** el reporte Markdown. Ese stdout es lo que un bloque
  futuro de integración con la GitHub Action publicará como comentario de PR, así
  que no debe contener nada más.
- **stderr** → todos los mensajes de error (`config inválido`, `error de escáner`,
  `error escribiendo SARIF`) y, cuando el build falla (`exit 1`), la razón
  (`result.Reason`) para que quede en los logs de CI.

## Por qué la lógica está separada de `main()`

`main()` con `os.Exit` disperso es imposible de testear. Aquí la lógica vive en
dos funciones:

- `exitCode(err error, shouldFail bool) int` — pura, sin efectos.
- `runApp(cfgPath, sarifOutPath string, loadConfig ..., runOrchestrator ..., writeFile ..., stdout io.Writer) int`
  — ejecuta el flujo completo con **todas sus dependencias inyectadas**
  (cargar config, correr orquestador, escribir archivo, y el `io.Writer` de
  salida), y devuelve el exit code en vez de llamar a `os.Exit`.

`main()` y el `RunE` de Cobra son *wiring* mínimo: construyen las
implementaciones reales (`config.Load`, `realOrchestrator`, un wrapper de
`os.WriteFile`, `os.Stdout`), llaman a `runApp`, y hacen `os.Exit(code)` con lo
que devuelva.

Así los tests (`cmd/pipelineguard/main_test.go`) ejercen todo el flujo con
closures y un `bytes.Buffer` como stdout, sin config real, sin escáneres reales y
sin tocar el disco:

- `exitCode`: los 3 casos (`nil`+`false`→0, `nil`+`true`→1, `error`→2).
- `loadConfig` devuelve error → exit `2`, y el orquestador **no** se invoca
  (verificado con un flag capturado por la closure).
- `runOrchestrator` devuelve error → exit `2`, y el SARIF **no** se escribe.
- Todo OK con `ShouldFail=true` → exit `1`, y `writeFile` recibió la ruta y los
  bytes correctos del SARIF.
- Todo OK con `ShouldFail=false` → exit `0`, y el Markdown aparece en el stdout
  capturado.
- `writeFile` falla (disco lleno simulado) → exit `2` aunque el escaneo haya sido
  exitoso, y el Markdown **no** se imprime (se aborta antes).
- Los defaults de los flags `--config` y `--sarif-output`.

## Cómo compilar y probar

```sh
go build ./...                 # compila el binario
go test ./cmd/pipelineguard/...
```

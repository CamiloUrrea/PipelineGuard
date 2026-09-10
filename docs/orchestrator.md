# Orquestación end-to-end (`internal/orchestrator`)

El paquete `internal/orchestrator` conecta todas las piezas de PipelineGuard en
un solo flujo: corre los escáneres habilitados, normaliza y combina sus
hallazgos, calcula el risk score, renderiza los reportes Markdown y SARIF, y
consulta a `internal/policy` si el build debe fallar.

Es el "pegamento" entre los paquetes de bloques anteriores. **No** ejecuta
binarios reales ni escribe archivos a disco: eso es trabajo de
`cmd/pipelineguard` (Bloque 9).

## Los escáneres se inyectan como funciones

```go
type ScannerFunc func() ([]byte, error)

func Run(cfg config.Config, runGitleaks, runTrivy ScannerFunc) (Result, error)
```

`Run` no sabe *cómo* se obtiene la salida de gitleaks o trivy: recibe dos
`ScannerFunc` que, al invocarse, devuelven los bytes crudos del reporte (los
mismos que la CLI escribiría en stdout) o un error.

**Por qué:** así toda la lógica de conexión —qué escáner correr, cómo parsear,
cómo combinar, cuándo fallar— se puede testear con *closures* que devuelven
bytes fijos o errores simulados, **sin depender de que gitleaks/trivy estén
instalados**. En `cmd/pipelineguard` estas funciones envolverán `os/exec`; en los
tests son funciones de una línea.

## `Result`

```go
type Result struct {
    Findings   []parsers.Finding
    Score      int
    Counts     map[string]int
    Markdown   string
    SARIF      []byte
    ShouldFail bool
    Reason     string
}
```

Todo lo que produce una corrida, en una sola estructura. El SARIF se devuelve
como `[]byte` (no se guarda a disco aquí).

## Flujo de `Run`

1. **gitleaks** — si `cfg.Scanners.Gitleaks` es `true`:
   - llama a `runGitleaks()`. Si devuelve error → `Run` devuelve
     `fmt.Errorf("running gitleaks scanner: %w", err)`.
   - si no, parsea con `parsers.ParseGitleaks`. Si el parseo falla → `Run`
     devuelve `fmt.Errorf("parsing gitleaks output: %w", err)`.
   - los `Finding` resultantes se acumulan.
   - si `cfg.Scanners.Gitleaks` es `false`, **`runGitleaks` no se invoca nunca**.
2. **trivy** — mismo patrón exacto con `cfg.Scanners.Trivy`, `runTrivy` y
   `parsers.ParseTrivy` (mensajes `running trivy scanner` / `parsing trivy output`).
3. **semgrep** — `cfg.Scanners.Semgrep` se **ignora por completo** en este
   bloque, aunque esté en `true`. No está implementado (es v1.1). No es error ni
   warning.
4. `score := scoring.ComputeScore(findings, cfg.SeverityWeights)`
5. `counts := scoring.CountBySeverity(findings)`
6. `markdown := report.GenerateMarkdown(findings, score, counts)`
7. `sarifBytes, err := report.GenerateSARIF(findings)` — si falla, `Run` devuelve
   `fmt.Errorf("generating SARIF report: %w", err)`.
8. `shouldFail, reason := policy.ShouldFail(findings, cfg)`
9. Devuelve el `Result` poblado y `error == nil`.

Cuando `Run` devuelve un error, el `Result` que lo acompaña es el valor cero
(`Result{}`).

## "Fallar ruidoso" ante errores de escaneo

Un error de un `ScannerFunc` significa que **el escaneo en sí falló** (el binario
no arrancó, salió con código distinto de cero, la salida no se pudo parsear...).
Eso **nunca** se trata como "no se encontraron hallazgos":

- Si se tragara el error, un gitleaks roto en CI se vería como "0 secretos" —
  un falso negativo silencioso, justo lo que una herramienta de seguridad no
  debe hacer.
- Por eso `Run` aborta de inmediato y propaga el error envuelto con `%w` y con
  contexto de **qué escáner y qué paso** falló, para que el llamador
  (`cmd/pipelineguard`) pueda decidir salir con código de error y un mensaje
  claro.

## Cómo correr los tests

```sh
go test ./internal/orchestrator/...
```

`orchestrator_test.go` usa `ScannerFunc` de prueba (closures), nunca gitleaks o
trivy reales, y cubre:

- Ambos escáneres habilitados con JSON válido → `Result` combina los `Finding` de
  ambas fuentes.
- `cfg.Scanners.Gitleaks=false` → `runGitleaks` no se invoca (verificado con un
  flag capturado por la closure) y no hay hallazgos de gitleaks. Ídem para trivy.
- `runGitleaks` devuelve error simulado → `Run` devuelve error que menciona
  `gitleaks`, y `Result` es el valor cero. Ídem para trivy.
- `runGitleaks` devuelve bytes no-JSON → `Run` devuelve error (desde
  `ParseGitleaks`), sin panic.
- Ambos escáneres deshabilitados → `Result` con `Findings` vacío, `Score=0`, sin
  error (el SARIF sigue siendo un documento válido vacío).
- `cfg.Scanners.Semgrep=true` con el resto deshabilitado → ignorado, sin error.
- `cfg.Enforce=true` con un hallazgo que supera el umbral → `Result.ShouldFail` y
  `Result.Reason` coinciden con lo que devuelve `policy.ShouldFail` directamente.

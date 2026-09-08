# Scoring

El paquete `internal/scoring` convierte una lista de hallazgos ya normalizados
(`[]parsers.Finding`) en números agregados: un **risk score** ponderado y un
**conteo por severidad**. No recibe JSON crudo ni valida nada — eso ya lo hizo
`internal/parsers`.

## Pesos por defecto

`var DefaultSeverityWeights map[string]int` define el peso de cada severidad en el
risk score (ver `ARCHITECTURE.md`, "Modelo de severidad"):

| Severidad  | Peso |
|------------|------|
| `CRITICAL` | 10   |
| `HIGH`     | 5    |
| `MEDIUM`   | 2    |
| `LOW`      | 1    |
| `INFO`     | 0    |

## `scoring.go`

### `ComputeScore(findings []parsers.Finding, weights map[string]int) int`

Suma el peso correspondiente a la severidad de cada `Finding`.

- El mapa de `weights` se recibe como parámetro; pasa `DefaultSeverityWeights`
  para el comportamiento por defecto, o un mapa personalizado para sobreescribirlo.
- Si la severidad de un `Finding` **no** es una llave en `weights`, aporta `0` al
  score (el `Finding` igual se recorre; simplemente no suma). No falla ni entra en
  panic.
- Un slice vacío o `nil` devuelve `0`.

### `CountBySeverity(findings []parsers.Finding) map[string]int`

Devuelve cuántos `Finding` hay de cada severidad **presente**.

- Las severidades sin hallazgos no aparecen en el mapa (ej. `{"CRITICAL": 3, "MEDIUM": 1}`).
- Cuenta cualquier valor de severidad, incluso uno no contemplado en los pesos.
- El mapa devuelto es no-`nil` aunque `findings` esté vacío (queda vacío, sin entradas).

### Mapeo Finding → score

| Entrada                    | Uso                                                         |
|----------------------------|------------------------------------------------------------|
| `Finding.Severity`         | Llave en `weights` (para el score) y en el conteo.         |
| resto de campos de `Finding` | Ignorados por este paquete.                              |
| `weights[severity]` ausente | Contribuye `0` al score.                                   |

## Cómo correr los tests

```sh
go test ./internal/scoring/...
```

Los tests (`scoring_test.go`) usan `testing` + `testify/assert` y cubren:

- Slice vacío → `ComputeScore` = 0, `CountBySeverity` = mapa vacío (no `nil`).
- Un `Finding` de cada severidad conocida → score = suma correcta con `DefaultSeverityWeights`.
- Múltiples `Finding` de la misma severidad → se cuentan y ponderan todos.
- Una severidad ausente en el mapa de `weights` → aporta 0, sin error ni panic
  (pero sí se cuenta en `CountBySeverity`).
- Un mapa de `weights` personalizado → `ComputeScore` lo respeta en vez del default.

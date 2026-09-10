# Decisión de enforcement (`ShouldFail`)

El paquete `internal/policy` decide si una corrida de PipelineGuard **debe hacer
fallar el build**, a partir de los hallazgos ya normalizados
(`[]parsers.Finding`) y la configuración ya cargada y validada
(`config.Config`). No hace ninguna E/S y no devuelve `error`: los datos de
entrada ya vienen validados por `internal/parsers` e `internal/config`.

> Aplicar de verdad esta decisión (código de salida del proceso, comentario en el
> PR) es trabajo de `cmd/pipelineguard`, un bloque posterior. Aquí solo se
> calcula el veredicto.

## `parsers.SeverityRank(severity string) int`

Traduce el enum de severidad unificado a un rango comparable, de más severo a
menos severo:

| Severidad   | Rank |
|-------------|------|
| `CRITICAL`  | 4    |
| `HIGH`      | 3    |
| `MEDIUM`    | 2    |
| `LOW`       | 1    |
| `INFO`      | 0    |
| cualquier valor no reconocido | 0 |

Lo desconocido se trata como **lo menos severo** (igual que `INFO`), nunca como
lo más severo: así un valor inesperado no puede disparar un fallo de build por sí
solo. La comparación es *case-sensitive* (`critical` → 0).

Vive en `internal/parsers/severity.go`. Es una función independiente y no
modifica `Finding`, `ParseGitleaks` ni `ParseTrivy`.

> Nota: `internal/report` mantiene su propio orden de severidad local para
> presentación. Es una duplicación aceptada a propósito; puntuar/ordenar para
> mostrar y decidir enforcement son conceptos distintos y no se acoplan.

## `policy.ShouldFail(findings []parsers.Finding, cfg config.Config) (fail bool, reason string)`

`reason` **nunca está vacío**: siempre explica el veredicto.

### Modo informativo — `cfg.Enforce == false`

Devuelve siempre:

```
false, "modo informativo: enforce=false, el build no falla independientemente de los hallazgos"
```

No importa cuántos hallazgos `CRITICAL` haya.

### Modo enforcement — `cfg.Enforce == true`

1. Calcula `threshold := parsers.SeverityRank(cfg.FailThreshold)`.
   (`config.Load` ya garantiza que `FailThreshold` es `CRITICAL`, `HIGH` o
   `MEDIUM`.)
2. Cuenta los hallazgos con `parsers.SeverityRank(f.Severity) >= threshold`
   (el propio umbral cuenta: la comparación es `>=`, no `>`).
3. Si el conteo es `> 0`:

   ```
   true, "enforce=true: <n> hallazgo(s) igualan o superan el umbral <FailThreshold>; el build falla"
   ```

4. Si ningún hallazgo alcanza el umbral (incluye la lista vacía):

   ```
   false, "enforce=true: ningún hallazgo alcanzó el umbral <FailThreshold>; el build no falla"
   ```

## Ejemplos de escenarios

| `Enforce` | `FailThreshold` | Severidades de los hallazgos | `fail` | Motivo |
|-----------|-----------------|------------------------------|--------|--------|
| `false`   | `CRITICAL`      | `[CRITICAL, CRITICAL]`       | `false`| Modo informativo: nunca falla. |
| `true`    | `CRITICAL`      | `[LOW, CRITICAL]`            | `true` | 1 hallazgo iguala el umbral (rank 4 ≥ 4). |
| `true`    | `CRITICAL`      | `[HIGH, MEDIUM, LOW]`        | `false`| Ninguno llega a rank 4. |
| `true`    | `MEDIUM`        | `[HIGH]`                     | `true` | `HIGH` (rank 3) supera `MEDIUM` (rank 2). |
| `true`    | `MEDIUM`        | `[MEDIUM]`                   | `true` | Coincidencia exacta: `>=` incluye el umbral. |
| `true`    | `CRITICAL`      | `[]` (vacío)                 | `false`| No hay hallazgos que evaluar. |

## Cómo correr los tests

```sh
go test ./internal/parsers/... ./internal/policy/...
```

- `internal/parsers/severity_test.go`: cada severidad conocida mapea a su rank,
  el orden estricto `CRITICAL > HIGH > MEDIUM > LOW > INFO`, y valores
  inventados / vacíos / en minúscula → 0.
- `internal/policy/policy_test.go`: modo informativo con `CRITICAL` presente →
  `false`; enforcement con y sin hallazgo que alcanza el umbral; umbral `MEDIUM`
  con un `HIGH` más severo; coincidencia exacta; lista vacía → `false`; y que
  `reason` no está vacío en ningún escenario.

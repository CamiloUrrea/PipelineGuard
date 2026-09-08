# Report

El paquete `internal/report` renderiza una lista de hallazgos ya normalizados
(`[]parsers.Finding`), junto con el risk score y el conteo por severidad ya
calculados por `internal/scoring`, en un reporte **Markdown** pensado para
publicarse como comentario de Pull Request.

No hace ninguna llamada a la API de GitHub — solo devuelve el string.

## `report.go`

### `GenerateMarkdown(findings []parsers.Finding, score int, countsBySeverity map[string]int) string`

Estructura del Markdown generado:

1. **Título + risk score**, siempre presente:
   `## 🛡️ PipelineGuard — Risk Score: <score>`
2. **Tabla resumen de conteo por severidad**: solo si `countsBySeverity` no está
   vacío. Se listan únicamente las severidades presentes en el mapa, en el orden
   fijo `CRITICAL, HIGH, MEDIUM, LOW, INFO`. Si el mapa está vacío, la tabla se
   omite por completo.
3. **Si `findings` está vacío**: la línea `✅ No se encontraron hallazgos.`
   (no se genera ninguna tabla de detalle vacía).
4. **Si hay `findings`**: una tabla de detalle con columnas
   `Severidad | Herramienta | Archivo:Línea | Regla | Mensaje`, ordenada por
   severidad (mismo orden fijo, `CRITICAL` primero) y, dentro de una misma
   severidad, por nombre de archivo (`File`) ascendente.

### Orden de despliegue

```go
var severityDisplayOrder = []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "INFO"}
```

Es un slice local del paquete, usado solo para ordenar la presentación. **No** se
importan ni reutilizan los pesos de `internal/scoring` (puntuar y ordenar son
conceptos distintos y no se acoplan). Una severidad no reconocida se ordena
después de todas las conocidas.

### Mapeo Finding → fila de la tabla de detalle

| Columna         | Origen                        |
|-----------------|-------------------------------|
| `Severidad`     | `Finding.Severity`            |
| `Herramienta`   | `Finding.Tool`                |
| `Archivo:Línea` | `Finding.File` + `":"` + `Finding.Line` |
| `Regla`         | `Finding.Rule`                |
| `Mensaje`       | `Finding.Message`             |

## Ejemplo de salida

Con dos hallazgos (uno de gitleaks, uno de trivy) y `score = 12`:

```markdown
## 🛡️ PipelineGuard — Risk Score: 12

| Severidad | Hallazgos |
|---|---|
| CRITICAL | 1 |
| MEDIUM | 1 |

| Severidad | Herramienta | Archivo:Línea | Regla | Mensaje |
|---|---|---|---|---|
| CRITICAL | gitleaks | config/settings.py:12 | generic-api-key | Generic API Key |
| MEDIUM | trivy | go.sum:0 | CVE-2023-99999 | example: information disclosure via verbose errors (github.com/example/leaky-lib@0.5.0) |
```

Sin hallazgos y `score = 0`:

```markdown
## 🛡️ PipelineGuard — Risk Score: 0

✅ No se encontraron hallazgos.
```

## Cómo correr los tests

```sh
go test ./internal/report/...
```

Los tests (`report_test.go`) usan `testing` + `testify/assert` y cubren:

- `findings` vacío → contiene el mensaje "sin hallazgos" y el score, sin tabla de
  detalle vacía.
- Un `Finding` → aparece en la tabla con todos sus campos.
- Severidades mezcladas → orden correcto verificado con `strings.Index`
  (posiciones relativas, no comparación de string completo).
- Misma severidad → filas ordenadas por nombre de archivo.
- `countsBySeverity` vacío → no se genera la tabla resumen.

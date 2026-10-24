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
   severidad, por nombre de archivo (`File`) ascendente. Como mucho
   `maxDetailRows` filas; si se omite alguna, un aviso debajo de la tabla (ver
   "Truncado del comentario").

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

### Escape de `|` en las celdas

Un `|` literal dentro de un campo (p. ej. un `Message` como `path A|path B`)
se leería como separador de columna y rompería la fila. Por eso **todas** las
celdas de texto de la tabla de detalle (`Severidad`, `Herramienta`, `File`,
`Regla`, `Mensaje`) pasan por:

```go
func escapeTableCell(s string) string // reemplaza "|" por "\|"
```

`path A|path B` se renderiza como `path A\|path B`, y la fila conserva sus 5
columnas.

### Truncado del comentario (Bloque 22)

GitHub rechaza el cuerpo de un comentario de más de **65.536 caracteres**, y un
repo con muchos hallazgos (o mensajes muy largos) podía superarlo: el comentario
no se publicaba. Dos límites, ambos constantes del paquete:

```go
const maxDetailRows = 100       // filas máximas en la tabla de detalle
const maxMarkdownChars = 60000  // red de seguridad para el Markdown completo
```

1. **Tope de filas**: la tabla de detalle muestra solo las primeras
   `maxDetailRows` filas **después** de ordenar, así que lo que se omite es
   siempre lo de menor severidad (y, dentro de la misma severidad, los últimos
   archivos en orden alfabético).
2. **Red de seguridad**: si con esas filas el Markdown completo (título, tabla
   resumen, tabla de detalle y aviso) aún supera `maxMarkdownChars`, se quitan
   filas del final, una a una, hasta que quepa. Se cuentan **caracteres**
   (`utf8.RuneCountInString`), no bytes, porque así mide GitHub su límite: un
   mensaje con `é` o emojis ocupa más bytes que caracteres. El margen hasta
   65.536 cubre el salto de línea que agrega `fmt.Fprintln` en `cmd/pipelineguard`
   y cualquier texto que se sume al publicar.
3. **Aviso**: cuando se omite al menos una fila, debajo de la tabla aparece:

   ```markdown
   > ⚠️ Se muestran 100 de 250 hallazgos (los de mayor severidad); se omitieron 150 para no exceder el límite de tamaño de los comentarios de GitHub. El detalle completo está en la pestaña **Security** del repositorio y en el archivo SARIF.
   ```

   Si la red de seguridad deja 0 filas (un solo mensaje más largo que el límite),
   se omite también el encabezado de la tabla y queda solo el aviso.

La tabla **resumen** de conteos **no** se trunca: siempre refleja el total real.
El **SARIF no se trunca nunca** (`GenerateSARIF` recibe todos los hallazgos), y
el exit code y `policy.ShouldFail` tampoco cambian: el truncado es solo de
presentación.

> Estado: cubierto solo con pruebas unitarias. No se validó todavía en un repo
> real con más de 100 hallazgos.

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
- `escapeTableCell` como función pura (sin `|`, uno, varios, string vacío).
- Un `Finding` con `|` en `Message`, `File` y `Rule` → aparecen escapados como
  `\|`, y la fila tiene el mismo número de `|` sin escapar (separadores reales)
  que el encabezado: 6, es decir, 5 columnas.
- Truncado (Bloque 22):
  - 0 hallazgos → sin filas de detalle y sin aviso.
  - Exactamente `maxDetailRows` → todas las filas, sin aviso.
  - `maxDetailRows + 1` con un único `LOW` (el primero en la entrada) → se
    omite justo ese `LOW`, las filas restantes son todas `CRITICAL` en orden
    por archivo, y el aviso dice `Se muestran 100 de 101` / `se omitieron 1`.
  - 10 hallazgos con mensajes de ~10.000 caracteres → la red de seguridad deja
    el reporte en ≤ `maxMarkdownChars` caracteres, la fila `HIGH` (la última
    tras ordenar) es la que cae, y el aviso cuenta bien las omitidas.
  - 5 mensajes de 10.000 `é` (~50.000 caracteres, ~100.000 bytes) → **no** se
    trunca nada: prueba que el límite cuenta caracteres, no bytes.

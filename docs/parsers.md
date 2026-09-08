# Parsers

El paquete `internal/parsers` traduce la salida cruda de cada escáner de seguridad
a un modelo compartido (`Finding`), de modo que el resto de PipelineGuard pueda
consolidar los hallazgos sin conocer los detalles de cada herramienta.

## `finding.go`

Define el struct compartido `Finding`, que es la representación normalizada de un
único hallazgo de seguridad:

| Campo      | Tipo     | Descripción                                                        |
|------------|----------|-------------------------------------------------------------------|
| `Tool`     | `string` | Escáner que produjo el hallazgo, p. ej. `"gitleaks"`.             |
| `Severity` | `string` | Nivel normalizado: `CRITICAL` \| `HIGH` \| `MEDIUM` \| `LOW` \| `INFO`. |
| `File`     | `string` | Ruta del archivo (relativa al repo) donde se detectó el problema. |
| `Line`     | `int`    | Línea inicial (base 1). `0` significa "desconocida".              |
| `Rule`     | `string` | Identificador de la regla de la herramienta, p. ej. `generic-api-key`. |
| `Message`  | `string` | Descripción breve y legible del problema.                         |

## `gitleaks.go`

Expone `ParseGitleaks(raw []byte) ([]Finding, error)`.

Recibe el JSON crudo que produce `gitleaks detect --report-format json`
(gitleaks v8.x) y devuelve la lista de `Finding` normalizados.

- El reporte de gitleaks es un **array JSON**; cuando no hay hallazgos es `[]` y la
  función devuelve un slice vacío (no `nil`) sin error.
- Un JSON malformado devuelve un `error` (envuelto con `%w`), nunca produce panic.
- Solo se deserializan los campos que PipelineGuard necesita
  (`Description`, `StartLine`, `File`, `RuleID`); el resto del JSON se ignora.

### Mapeo de campos gitleaks → Finding

| Campo gitleaks | Campo `Finding` | Notas                                                             |
|----------------|-----------------|-------------------------------------------------------------------|
| —              | `Tool`          | Constante `"gitleaks"`.                                           |
| —              | `Severity`      | Constante `"CRITICAL"`. Gitleaks no emite severidad propia; según `ARCHITECTURE.md` todo secreto filtrado se trata como `CRITICAL`. |
| `File`         | `File`          | Ruta tal cual la reporta gitleaks.                               |
| `StartLine`    | `Line`          | Línea inicial del hallazgo.                                      |
| `RuleID`       | `Rule`          | Identificador de la regla, p. ej. `generic-api-key`.             |
| `Description`  | `Message`       | Texto descriptivo de la regla, p. ej. `Generic API Key`.         |

## `trivy.go`

Expone `ParseTrivy(raw []byte) ([]Finding, error)`.

Recibe el JSON crudo que produce `trivy fs --format json` (escaneo de
vulnerabilidades de dependencias, no de imágenes de contenedor) y devuelve la
lista de `Finding` normalizados.

- El reporte de trivy es un **objeto JSON** con una clave `Results` (array de
  objetivos escaneados).
- `Results` puede venir como `null` (no se escaneó nada): la función devuelve un
  slice vacío (no `nil`) sin error.
- Un objetivo dentro de `Results` puede no traer el campo `Vulnerabilities`
  (objetivo limpio): se ignora, sin error ni Findings.
- Un JSON malformado devuelve un `error` (envuelto con `%w`), nunca produce panic.
- Solo se deserializan los campos que PipelineGuard necesita (`Target`,
  `Vulnerabilities[].VulnerabilityID`, `PkgName`, `InstalledVersion`, `Title`,
  `Severity`); el resto del JSON se ignora (`SchemaVersion`, `Class`, `Type`,
  `FixedVersion`, `References`, etc.).

### Normalización de severidad

`normalizeTrivySeverity(sev string) string` mapea la severidad nativa de trivy al
enum unificado (ver `ARCHITECTURE.md`, "Modelo de datos" / "Modelo de severidad"):

| Severidad trivy | Severidad `Finding` |
|-----------------|---------------------|
| `CRITICAL`      | `CRITICAL`          |
| `HIGH`          | `HIGH`              |
| `MEDIUM`        | `MEDIUM`            |
| `LOW`           | `LOW`               |
| `UNKNOWN`       | `LOW`               |
| cualquier otro valor no reconocido | `LOW` (robustez futura) |

### Mapeo de campos trivy → Finding

| Campo trivy | Campo `Finding` | Notas                                                          |
|-------------|-----------------|---------------------------------------------------------------|
| —           | `Tool`          | Constante `"trivy"`.                                          |
| `Severity`  | `Severity`      | Normalizada con `normalizeTrivySeverity` (ver tabla arriba). |
| `Target`    | `File`          | Del `Result` que contiene la vulnerabilidad, p. ej. `go.sum`. |
| —           | `Line`          | Siempre `0`; trivy no reporta línea para vulns de dependencias (`0` = "desconocida", ver `finding.go`). |
| `VulnerabilityID` | `Rule`    | Identificador del CVE/GHSA, p. ej. `CVE-2024-12345`.         |
| `Title`, `PkgName`, `InstalledVersion` | `Message` | `fmt.Sprintf("%s (%s@%s)", Title, PkgName, InstalledVersion)`. |

## Fixtures de pruebas

- `internal/parsers/testdata/gitleaks_sample.json` — 3 hallazgos de ejemplo
  (API key genérica, AWS access key y GitHub PAT).
- `internal/parsers/testdata/trivy_sample.json` — un `Result` (`go.sum`) con 3
  vulnerabilidades de severidades distintas (CRITICAL, MEDIUM, UNKNOWN) y un
  segundo `Result` (`package-lock.json`) sin `Vulnerabilities` (objetivo limpio).

Todos los valores son ficticios: no contienen credenciales ni datos reales.

## Cómo correr los tests

```sh
go test ./internal/parsers/...
```

Los tests usan `testing` + `testify/assert`/`require`.

`gitleaks_test.go` cubre:

- El mapeo correcto de cada campo leyendo el fixture.
- Que todos los hallazgos de gitleaks quedan como `CRITICAL` / `Tool="gitleaks"`.
- El caso de array vacío `[]` (slice vacío, no `nil`, sin error).
- El caso de JSON malformado (devuelve `error`, no panic).

`trivy_test.go` cubre:

- `normalizeTrivySeverity` de forma independiente (incluye `UNKNOWN`→`LOW` y un
  valor inventado→`LOW`).
- El mapeo correcto de cada campo leyendo el fixture.
- La agregación de múltiples `Results`.
- Un `Result` sin `Vulnerabilities` (no genera error ni Findings).
- `Results: null` y `Results` ausente (slice vacío, sin error).
- El caso de JSON malformado (devuelve `error`, no panic).

> Nota: si es la primera vez que se compila el módulo, ejecuta antes `go mod tidy`
> para descargar `testify` y sus dependencias.

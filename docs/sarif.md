# SARIF

El paquete `internal/report` también genera, además del Markdown, un documento
**SARIF 2.1.0** (formato estándar de OASIS que GitHub lee nativamente en la
pestaña *Security*). Igual que `GenerateMarkdown`, no hace ninguna llamada a la
API de GitHub: solo devuelve los bytes.

## `sarif.go`

### `GenerateSARIF(findings []parsers.Finding) ([]byte, error)`

Recibe la lista de hallazgos ya normalizados y devuelve el JSON del documento
SARIF (indentado con dos espacios). Solo usa la librería estándar
(`encoding/json`).

El error solo puede provenir de `json.MarshalIndent` (envuelto con `%w`); con
entradas normales nunca falla y jamás produce panic.

## Estructura del documento

```json
{
  "$schema": "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
  "version": "2.1.0",
  "runs": [
    {
      "tool": {
        "driver": {
          "name": "PipelineGuard",
          "rules": [ /* deduplicadas por Rule */ ]
        }
      },
      "results": [ /* un elemento por Finding */ ]
    }
  ]
}
```

Siempre hay exactamente un `run`, con `tool.driver.name = "PipelineGuard"`.

## Mapeo de severidad → `level` SARIF

| `Finding.Severity`            | `level` SARIF |
|-------------------------------|---------------|
| `CRITICAL`                    | `error`       |
| `HIGH`                        | `error`       |
| `MEDIUM`                      | `warning`     |
| `LOW`                         | `note`        |
| `INFO`                        | `none`        |
| cualquier otro valor no reconocido | `warning` (punto medio: ni se ignora ni se sobre-alarma) |

## Deduplicación de `rules`

El array `tool.driver.rules` se deduplica por `Finding.Rule`: aunque haya varios
hallazgos con la misma regla, `rules` contiene una sola entrada para ese
`ruleId`. El texto de `shortDescription.text` se toma del `Message` del **primer**
`Finding` que use esa regla; los `Message` de hallazgos posteriores con la misma
regla no alteran la entrada (siguen apareciendo, eso sí, como `message.text` de
su propio `result`).

Cada `Finding` siempre produce su propio elemento en `results`, con independencia
de la deduplicación de `rules`.

## Mapeo `Finding` → `result`

| Campo del `result`                                   | Origen                                  |
|------------------------------------------------------|-----------------------------------------|
| `ruleId`                                             | `Finding.Rule`                          |
| `level`                                              | `Finding.Severity` (tabla de arriba)    |
| `message.text`                                       | `Finding.Message`                       |
| `properties.originTool`                              | `Finding.Tool` (`gitleaks` / `trivy`), para no perder la trazabilidad de qué escáner originó el resultado |
| `locations[0].physicalLocation.artifactLocation.uri` | `Finding.File`                          |
| `locations[0].physicalLocation.region.startLine`     | `Finding.Line`, **solo si `Line > 0`**  |

### Línea desconocida (`Line == 0`)

Según la convención documentada en `finding.go`, `Line == 0` significa "línea
desconocida" (típico de las vulnerabilidades de dependencias de trivy). SARIF
espera `startLine >= 1`, así que en ese caso se **omite el objeto `region`
completo** dentro de `physicalLocation` en vez de enviar un `startLine: 0`
inválido.

### Sin hallazgos

Si `findings` está vacío, el documento sigue siendo JSON válido con
`"results": []` y `"rules": []` (arrays vacíos, nunca `null`).

## Ejemplo real de salida

Con dos hallazgos (uno de gitleaks con línea conocida, uno de trivy sin línea):

```json
{
  "$schema": "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
  "version": "2.1.0",
  "runs": [
    {
      "tool": {
        "driver": {
          "name": "PipelineGuard",
          "rules": [
            {
              "id": "generic-api-key",
              "shortDescription": {
                "text": "Generic API Key"
              }
            },
            {
              "id": "CVE-2023-99999",
              "shortDescription": {
                "text": "example: information disclosure (github.com/example/leaky-lib@0.5.0)"
              }
            }
          ]
        }
      },
      "results": [
        {
          "ruleId": "generic-api-key",
          "level": "error",
          "message": {
            "text": "Generic API Key"
          },
          "properties": {
            "originTool": "gitleaks"
          },
          "locations": [
            {
              "physicalLocation": {
                "artifactLocation": {
                  "uri": "config/settings.py"
                },
                "region": {
                  "startLine": 12
                }
              }
            }
          ]
        },
        {
          "ruleId": "CVE-2023-99999",
          "level": "warning",
          "message": {
            "text": "example: information disclosure (github.com/example/leaky-lib@0.5.0)"
          },
          "properties": {
            "originTool": "trivy"
          },
          "locations": [
            {
              "physicalLocation": {
                "artifactLocation": {
                  "uri": "go.sum"
                }
              }
            }
          ]
        }
      ]
    }
  ]
}
```

Sin hallazgos:

```json
{
  "$schema": "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
  "version": "2.1.0",
  "runs": [
    {
      "tool": {
        "driver": {
          "name": "PipelineGuard",
          "rules": []
        }
      },
      "results": []
    }
  ]
}
```

## Cómo correr los tests

```sh
go test ./internal/report/...
```

Los tests (`sarif_test.go`) usan `testing` + `testify/assert`/`require` y
verifican el resultado con `json.Unmarshal` (no comparan el string completo):

- `findings` vacío → JSON válido, `results` y `rules` son arrays vacíos (no `null`).
- Un `Finding` con `Line > 0` → `region.startLine` presente y correcto.
- Un `Finding` con `Line == 0` → sin campo `region`.
- Cada severidad (`CRITICAL`, `HIGH`, `MEDIUM`, `LOW`, `INFO`) mapea a su `level`;
  un valor de severidad inventado → `warning`.
- Varios `Finding` con el mismo `Rule` → una sola entrada en `rules`, con el
  `Message` del primero.
- `properties.originTool` refleja el `Tool` de cada `Finding`.

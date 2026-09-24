# Configuración (`.pipelineguard.yml`)

El paquete `internal/config` carga el archivo **opcional** `.pipelineguard.yml`
del repositorio y lo convierte en un `Config` tipado. Solo entiende YAML: no hay
un cargador genérico multi-formato.

## API

### `DefaultConfig() Config`

Devuelve la configuración por defecto, que se usa cuando el archivo no existe y,
campo por campo, para cualquier valor que el archivo no especifique.

| Campo             | Valor por defecto                                   |
|-------------------|----------------------------------------------------|
| `Scanners.Gitleaks` | `true`                                           |
| `Scanners.Trivy`    | `true`                                           |
| `Scanners.Semgrep`  | `false`                                          |
| `Enforce`         | `false`                                            |
| `FailThreshold`   | `"CRITICAL"`                                        |
| `IgnorePaths`     | `nil`                                              |
| `SeverityWeights` | copia de `scoring.DefaultSeverityWeights` (`CRITICAL:10, HIGH:5, MEDIUM:2, LOW:1, INFO:0`) |

`SeverityWeights` es una **copia** del mapa de `internal/scoring`, no una
referencia: ni los llamadores ni el merge de YAML pueden mutar el mapa
compartido del paquete `scoring`.

### `Load(path string) (Config, error)`

1. **El archivo no existe** (`os.IsNotExist`): devuelve `DefaultConfig()` y
   `error == nil`. El archivo de configuración es opcional.
2. **El archivo existe**: parte de `cfg := DefaultConfig()` y aplica
   `yaml.Unmarshal(raw, &cfg)` encima. Los campos que el YAML no menciona
   conservan su valor por defecto (ver "Comportamiento de merge").
3. **Validación**: `FailThreshold` debe ser exactamente uno de `"CRITICAL"`,
   `"HIGH"` o `"MEDIUM"` (**case-sensitive**: `critical` es inválido). Si no,
   devuelve un error descriptivo que incluye el valor recibido.
4. **YAML malformado**: devuelve un `error` envuelto con `%w`. Nunca hace panic.

En cualquier caso de error, el `Config` devuelto es el valor cero (`Config{}`).

## Esquema de `.pipelineguard.yml`

```yaml
scanners:
  gitleaks: true
  trivy: true
  semgrep: false        # reservado para v1.1: hoy se acepta pero se ignora

enforce: false           # si es true, el build falla si ALGÚN hallazgo alcanza fail_threshold
fail_threshold: CRITICAL # CRITICAL | HIGH | MEDIUM

severity_weights:
  CRITICAL: 10
  HIGH: 5
  MEDIUM: 2
  LOW: 1
  INFO: 0
```

| Clave YAML         | Tipo             | Descripción                                                                 |
|--------------------|------------------|----------------------------------------------------------------------------|
| `scanners.gitleaks`| `bool`           | Ejecutar gitleaks (detección de secretos).                                  |
| `scanners.trivy`   | `bool`           | Ejecutar trivy (vulnerabilidades de dependencias).                          |
| `scanners.semgrep` | `bool`           | Reservado para v1.1 (SAST). Se **acepta** en el YAML, pero `internal/orchestrator` lo **ignora**: con `true` no corre semgrep ni da error. |
| `enforce`          | `bool`           | Si es `true`, el build falla cuando **algún** hallazgo tiene severidad **igual o mayor** que `fail_threshold`. Implementado en `policy.ShouldFail` (ver `docs/policy.md`); `cmd/pipelineguard` lo traduce en el exit code `1` (ver `docs/cmd.md`). Con `false` (default) el modo es informativo y nunca falla. |
| `fail_threshold`   | `string`         | Umbral de **severidad** para el enforcement: `CRITICAL` \| `HIGH` \| `MEDIUM`. Se compara hallazgo por hallazgo; **no** tiene relación con el risk score total. |
| `severity_weights` | `map[string]int` | Peso de cada severidad en el risk score. Sobreescribe los pesos por defecto llave por llave. |

### `ignore_paths`: aceptado, pero **todavía no se aplica** (reservado para v1.1)

`Config` tiene el campo `IgnorePaths` (`ignore_paths` en el YAML, `[]string`), y
`Load` lo parsea y lo guarda tal cual. **Ningún código lo usa**: no se filtran
hallazgos ni rutas, y no se pasa a gitleaks ni a trivy. Escribir
`ignore_paths` en `.pipelineguard.yml` **no tiene ningún efecto** en v1.0: los
hallazgos en esas rutas siguen apareciendo en el reporte, en el SARIF y en la
decisión de enforcement. Por eso no aparece en el ejemplo de arriba. Queda
reservado para v1.1.

## Comportamiento de merge con los defaults

`Load` arranca siempre desde `DefaultConfig()` y deja que `yaml.v3` decodifique
encima. Verificado empíricamente en `config_test.go`:

- **Campos de nivel superior**: un YAML con solo `enforce: true` deja todo lo
  demás igual a `DefaultConfig()`.
- **Bloque anidado `scanners`**: un YAML con solo `scanners: {gitleaks: false}`
  pone `Gitleaks = false` y **conserva** `Trivy = true` y `Semgrep = false`
  (yaml.v3 decodifica sobre el struct existente sin tocar los campos ausentes).
- **Mapa `severity_weights`**: un YAML con solo `severity_weights: {CRITICAL: 20}`
  cambia únicamente `CRITICAL` y **conserva** los pesos por defecto de `HIGH`,
  `MEDIUM`, `LOW` e `INFO` (yaml.v3 decodifica sobre el mapa existente sin
  vaciarlo).

Como este comportamiento de merge de yaml.v3 preserva los defaults tanto en el
struct anidado como en el mapa, **no hace falta merge manual campo por campo** en
`Load`. Los tests `TestLoad_NestedScannerMergePreservesDefaults` y
`TestLoad_SeverityWeightsMergePreservesDefaults` fijan esta garantía.

- **Slices (`ignore_paths`)**: yaml.v3 **reemplaza** el slice completo, no
  concatena. Como el default es `nil`, cualquier lista del YAML simplemente pasa
  a ser el valor.

## Cómo correr los tests

```sh
go test ./internal/config/...
```

Los tests (`config_test.go`) usan `testing` + `testify` y archivos de fixture en
un directorio temporal (`t.TempDir()`), nunca archivos reales en el repo.
Cubren: archivo ausente → defaults; YAML completo → todos los campos; YAML
parcial → defaults preservados; merge en `scanners` y en `severity_weights`;
que `scoring.DefaultSeverityWeights` no se muta por aliasing; `fail_threshold`
inválido y en minúsculas → error; y YAML malformado → error sin panic.

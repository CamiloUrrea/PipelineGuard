# PipelineGuard — Documento de Arquitectura

## 1. Visión general

**PipelineGuard** es una GitHub Action que orquesta escáneres de seguridad ya probados en la industria (gitleaks, trivy — semgrep en fase 2) y consolida sus resultados en **un único reporte normalizado, con severidad unificada y un risk score agregado**. Se instala en cualquier repositorio con una línea de configuración y corre automáticamente en cada Pull Request.

El valor del proyecto no está en la detección (eso ya lo hacen bien las herramientas subyacentes), sino en la **orquestación, normalización y experiencia de desarrollador**: un solo reporte legible en vez de tres formatos de salida distintos que nadie revisa.

## 2. Alcance

### MVP (v1)
- Integración con **gitleaks** (secretos) y **trivy** (vulnerabilidades de dependencias/contenedores).
- Normalización de hallazgos a un esquema común.
- Cálculo de risk score agregado.
- Comentario de resumen en el PR + archivo SARIF.
- Modo informativo por defecto (no bloquea el build).
- Distribución como composite action + binario en GitHub Releases.

### Fuera de alcance para v1 (roadmap)
- **v1.1** — integración con **semgrep** (SAST).
- **v1.2** — modo "diff-only": mostrar solo hallazgos *nuevos* introducidos por el PR, comparando contra un baseline.
- **v1.3** — soporte para Checkov/tfsec (IaC).
- **v1.4** — firma de binarios con cosign (Sigstore) para trazabilidad de la cadena de suministro.
- Futuro — dashboard web con tendencia histórica del risk score por repo.

## 3. Plataforma tecnológica

| Componente | Elección | Razón |
|---|---|---|
| Lenguaje | Go | Mismo lenguaje que gitleaks y trivy; binario único, rápido, sin runtime. |
| CLI framework | Cobra | Estándar de facto (usado por `gh`, `kubectl`). |
| Testing | `testing` + `testify/assert` | Librería estándar + aserciones legibles. |
| Logging | `log/slog` | Estructurado, sin dependencias externas (stdlib desde Go 1.21). |
| Release | GoReleaser | Compilación cruzada, checksums y changelog automatizados. |
| Distribución de la Action | Composite action (bash) + binario en GitHub Releases | Evita overhead de Docker; arranque casi instantáneo; funciona en Linux/macOS/Windows. |
| Licencia | MIT | Máxima libertad de uso, alineado con el objetivo de que "cualquiera lo use". |

## 4. Arquitectura del sistema

```
Repo del usuario
   └─ .github/workflows/*.yml (una línea: uses: <owner>/pipelineguard@v1)
        └─ composite action (bash)
             ├─ detecta OS/arch del runner
             ├─ descarga el binario correcto desde GitHub Releases
             ├─ verifica el checksum (checksums.txt de GoReleaser)
             └─ ejecuta el binario `pipelineguard`
                  ├─ corre gitleaks y trivy como subprocesos
                  ├─ parsea sus salidas (internal/parsers)
                  ├─ normaliza a esquema común (Finding)
                  ├─ calcula risk score (internal/scoring)
                  └─ genera (internal/report):
                       - comentario en el PR (vía API de GitHub, usando GITHUB_TOKEN)
                       - archivo SARIF (leído nativo por la pestaña Security de GitHub)
                       - código de salida (0 salvo que enforce=true y se supere el umbral)
```

## 5. Estructura del repositorio

Monorepo único, siguiendo el [Standard Go Project Layout](https://github.com/golang-standards/project-layout):

```
pipelineguard/
├── cmd/pipelineguard/       # entry point (main.go)
├── internal/
│   ├── parsers/             # parseo de salidas de gitleaks y trivy
│   │   └── testdata/        # fixtures JSON grabadas de corridas reales
│   ├── scoring/             # normalización de severidad + cálculo de risk score
│   └── report/              # generación de Markdown y SARIF
├── action/                  # action.yml + install.sh (composite action)
├── .goreleaser.yaml
├── .github/workflows/
│   ├── ci.yml                # lint + test + build, en cada PR
│   └── release.yml           # goreleaser release, en cada tag v*
├── CHANGELOG.md
├── LICENSE (MIT)
└── README.md
```

El repo de demostración (target vulnerable para pruebas de integración y capturas de portafolio) vive **separado**, como `pipelineguard-demo-vulnerable-app`, para no mezclar código inseguro-a-propósito con el código del producto.

## 6. Modelo de datos: normalización de hallazgos

Cada escáner reporta en su propio formato. Todo se normaliza a una estructura común antes de puntuar o reportar:

```go
type Finding struct {
    Tool     string // "gitleaks" | "trivy" | "semgrep"
    Severity string // enum unificado: CRITICAL | HIGH | MEDIUM | LOW | INFO
    File     string
    Line     int
    Rule     string
    Message  string
}
```

Mapeo de severidad nativa → enum unificado:

| Herramienta | Severidad nativa | Severidad unificada |
|---|---|---|
| trivy | CRITICAL / HIGH / MEDIUM / LOW / UNKNOWN | mapeo directo (UNKNOWN → LOW) |
| gitleaks | (no tiene severidad nativa; solo detecta presencia) | todo secreto detectado → **CRITICAL** por defecto (configurable) |

## 7. Modelo de severidad y comportamiento por defecto

- **Por defecto, PipelineGuard es informativo**: nunca falla el build, solo reporta.
- El equipo activa el enforcement explícitamente en `.pipelineguard.yml` (`enforce: true` + `fail_threshold`).
- Aun en modo informativo, los hallazgos **CRITICAL** se destacan visualmente con mayor énfasis en el comentario del PR (para no pasar desapercibidos sin forzar el bloqueo).
- Risk score = suma ponderada de hallazgos por severidad (pesos configurables, con default razonable: CRITICAL=10, HIGH=5, MEDIUM=2, LOW=1).

## 8. Configuración (`.pipelineguard.yml`)

```yaml
scanners:
  gitleaks: true
  trivy: true
  semgrep: false   # disponible desde v1.1

enforce: false        # si es true, falla el build al superar fail_threshold
fail_threshold: CRITICAL   # CRITICAL | HIGH | MEDIUM

ignore_paths:
  - "**/vendor/**"
  - "**/testdata/**"

severity_weights:
  CRITICAL: 10
  HIGH: 5
  MEDIUM: 2
  LOW: 1
```

## 9. Salidas

1. **Comentario en el PR** (Markdown): resumen ejecutivo, tabla de hallazgos agrupados por severidad, risk score total.
2. **Archivo SARIF**: subido con `github/codeql-action/upload-sarif`, visible nativamente en la pestaña *Security* del repo.
3. **Código de salida**: `0` salvo que `enforce: true` y el score supere `fail_threshold`.

## 10. Estrategia de testing

- **Unitarios**: parseo y scoring, usando fixtures JSON grabadas de salidas reales de gitleaks/trivy (sin ejecutar los binarios en cada test).
- **Integración**: un test que sí ejecuta los binarios reales contra `pipelineguard-demo-vulnerable-app` y valida el reporte final end-to-end.
- **CI del propio proyecto**: `ci.yml` corre lint (`golangci-lint`) + tests + build en cada PR.

## 11. Release y versionado

- SemVer estricto (`v1.0.0`, `v1.2.3`).
- Tag flotante `v1` apuntando siempre al último release de la serie 1.x, para que los usuarios puedan fijar `uses: <owner>/pipelineguard@v1`.
- Flujo completo automatizado con GoReleaser: build matrix multiplataforma → checksums → changelog (Conventional Commits) → publicación del GitHub Release → actualización del tag mayor.
- Roadmap fase 2: firma de binarios con cosign para trazabilidad de cadena de suministro.

## 12. Roadmap resumido

| Versión | Contenido |
|---|---|
| v1.0 | gitleaks + trivy, modo informativo, comentario en PR + SARIF |
| v1.1 | integración con semgrep |
| v1.2 | modo diff-only (solo hallazgos nuevos del PR) |
| v1.3 | soporte IaC (Checkov/tfsec) |
| v1.4 | firma de binarios con cosign |
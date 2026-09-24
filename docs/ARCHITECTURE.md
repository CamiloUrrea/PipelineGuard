# PipelineGuard — Documento de Arquitectura

## 1. Visión general

**PipelineGuard** es una GitHub Action que orquesta escáneres de seguridad ya probados en la industria (gitleaks, trivy — semgrep en fase 2) y consolida sus resultados en **un único reporte normalizado, con severidad unificada y un risk score agregado**. Se instala en cualquier repositorio con una línea de configuración y corre automáticamente en cada Pull Request.

El valor del proyecto no está en la detección (eso ya lo hacen bien las herramientas subyacentes), sino en la **orquestación, normalización y experiencia de desarrollador**: un solo reporte legible en vez de un formato de salida distinto por herramienta que nadie revisa.

## 2. Alcance

### MVP (v1)
- Integración con **gitleaks** (secretos) y **trivy** (vulnerabilidades de dependencias). Trivy corre en modo **filesystem** (`trivy fs .` sobre el directorio del repo), **no** contra imágenes de contenedor. PipelineGuard solo toma de su reporte las vulnerabilidades de dependencias (`Results[].Vulnerabilities`, a partir de manifiestos/lockfiles).
- Normalización de hallazgos a un esquema común.
- Cálculo de risk score agregado.
- Comentario de resumen en el PR + archivo SARIF.
- Modo informativo por defecto (no bloquea el build).
- Distribución como composite action + binario en GitHub Releases.

### Fuera de alcance para v1 (roadmap)
- **v1.1** — integración con **semgrep** (SAST) y aplicación real de `ignore_paths` (hoy se acepta en el YAML pero no se aplica).
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
| Release | GoReleaser | Compilación cruzada, checksums y changelog automatizados. |
| Distribución de la Action | Composite action (bash) + binario en GitHub Releases | Evita overhead de Docker; arranque casi instantáneo; funciona en Linux/macOS/Windows. |
| Licencia | MIT | Máxima libertad de uso, alineado con el objetivo de que "cualquiera lo use". |

## 4. Arquitectura del sistema

```
Repo del usuario
   └─ .github/workflows/*.yml (una línea: uses: CamiloUrrea/PipelineGuard/action@v1)
        └─ composite action (action/action.yml, 5 steps en bash)
             ├─ 1. install.sh
             │     ├─ resuelve "v1" a la release real más alta (API de Releases)
             │     ├─ detecta OS/arch del runner
             │     ├─ descarga el binario correcto desde GitHub Releases
             │     └─ verifica el checksum (checksums.txt de GoReleaser)
             ├─ 2. run.sh → ejecuta el binario `pipelineguard`
             │     ├─ corre gitleaks y trivy como subprocesos, con timeout (internal/scanners)
             │     ├─ parsea sus salidas y normaliza severidad a Finding (internal/parsers)
             │     ├─ calcula risk score (internal/scoring)
             │     ├─ decide enforce/fail_threshold (internal/policy)
             │     ├─ genera Markdown + SARIF (internal/report)
             │     └─ sale con 0 / 1 / 2 (ver §9); run.sh guarda ese código como output
             ├─ 3. upload-sarif → pestaña Security de GitHub
             ├─ 4. comment.sh → publica/actualiza el comentario del PR con `gh api`
             └─ 5. gate.sh → lee el exit code y decide si el job falla
```

El binario **no** habla con la API de GitHub. Solo escribe el SARIF a disco y
el Markdown a stdout. El comentario del PR lo publica `comment.sh` con
`gh api` (y `GH_TOKEN`), y la subida del SARIF la hace
`github/codeql-action/upload-sarif`.

## 5. Estructura del repositorio

Monorepo único, siguiendo el [Standard Go Project Layout](https://github.com/golang-standards/project-layout):

```
pipelineguard/
├── cmd/pipelineguard/       # entry point (main.go): flags, exit codes 0/1/2
├── internal/
│   ├── config/              # carga de .pipelineguard.yml + defaults
│   ├── parsers/             # parseo de gitleaks/trivy + normalización de severidad (Finding, SeverityRank)
│   │   └── testdata/        # fixtures JSON con la forma real de cada salida (valores ficticios)
│   ├── scoring/             # risk score ponderado + conteo por severidad
│   ├── policy/              # decisión de enforcement (ShouldFail)
│   ├── report/              # generación de Markdown y SARIF
│   ├── orchestrator/        # une todo: escáneres → parsers → scoring → report → policy
│   └── scanners/            # ejecución real de gitleaks/trivy (os/exec + timeout)
├── action/
│   ├── action.yml           # composite action (5 steps)
│   └── scripts/             # install.sh, run.sh, comment.sh, gate.sh + sus tests *_test.bats
├── docs/                    # documentación por componente (índice en docs/README.md)
├── .goreleaser.yaml
├── .github/workflows/
│   ├── ci.yml               # build/vet/gofmt/test + golangci-lint + shellcheck/bats, en cada PR y push a main
│   └── release.yml          # GoReleaser + tag flotante vN, en cada tag v*
├── go.mod / go.sum
├── LICENSE (MIT)
└── README.md
```

La normalización de severidad vive en `internal/parsers` (cada parser mapea la
severidad nativa al enum unificado, y `SeverityRank` la ordena), no en
`internal/scoring`, que solo suma pesos y cuenta.

El repo de demostración (target vulnerable para pruebas de integración y capturas de portafolio) vive **separado**, como `pipelineguard-demo-vulnerable-app`, para no mezclar código inseguro-a-propósito con el código del producto.

## 6. Modelo de datos: normalización de hallazgos

Cada escáner reporta en su propio formato. Todo se normaliza a una estructura común antes de puntuar o reportar:

```go
type Finding struct {
    Tool     string // "gitleaks" | "trivy" (semgrep llegará en v1.1)
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
| gitleaks | (no tiene severidad nativa; solo detecta presencia) | todo secreto detectado → **CRITICAL**, siempre (constante en `internal/parsers/gitleaks.go`; **no** es configurable) |

## 7. Modelo de severidad y comportamiento por defecto

- **Por defecto, PipelineGuard es informativo**: los **hallazgos** nunca hacen fallar el build, solo se reportan. Un **fallo de la herramienta** (exit code `2`) sí hace fallar el job también en modo informativo: un escaneo roto nunca se hace pasar por "0 hallazgos".
- El equipo activa el enforcement explícitamente en `.pipelineguard.yml` (`enforce: true` + `fail_threshold`). La regla es: el build falla si **algún hallazgo** tiene severidad **igual o mayor** que `fail_threshold` (`policy.ShouldFail`). El umbral se compara **hallazgo por hallazgo**, no contra el risk score total.
- En el comentario del PR, la tabla de detalle se ordena por severidad (**CRITICAL primero**) y hay una tabla resumen de conteos. No hay otro énfasis visual especial para los CRITICAL (ver `docs/report.md`).
- Risk score = suma ponderada de hallazgos por severidad (pesos configurables, default: CRITICAL=10, HIGH=5, MEDIUM=2, LOW=1, INFO=0). Es informativo: **no** interviene en la decisión de enforcement.
- **Exit codes del binario**: `0` = OK (o modo informativo), `1` = violación real (enforce + algún hallazgo ≥ umbral), `2` = fallo de la herramienta (config inválida, escáner que falla o supera el timeout, error de E/S, uso incorrecto de la CLI). En la Action, `gate.sh` convierte ese código en el resultado del job (ver `docs/cmd.md` y `docs/action-gate.md`).

## 8. Configuración (`.pipelineguard.yml`)

```yaml
scanners:
  gitleaks: true
  trivy: true
  semgrep: false   # reservado para v1.1: hoy se acepta pero se ignora

enforce: false             # si es true, falla si ALGÚN hallazgo alcanza fail_threshold
fail_threshold: CRITICAL   # CRITICAL | HIGH | MEDIUM

severity_weights:
  CRITICAL: 10
  HIGH: 5
  MEDIUM: 2
  LOW: 1
  INFO: 0
```

> `ignore_paths` (lista de globs) se **acepta** en el YAML, pero **no se aplica**
> en v1.0: ningún código filtra hallazgos ni rutas con él. Está reservado para
> v1.1 (ver `docs/config.md`).

## 9. Salidas

1. **Comentario en el PR** (Markdown): risk score total, tabla resumen de conteos por severidad y tabla de detalle ordenada por severidad. El binario lo genera a stdout; `run.sh` lo guarda en `pipelineguard-report.md` y `comment.sh` lo publica o actualiza vía `gh api` (un único comentario por PR).
2. **Archivo SARIF**: subido con `github/codeql-action/upload-sarif`, visible nativamente en la pestaña *Security* del repo.
3. **Código de salida**: `0` = OK (o `enforce: false`); `1` = `enforce: true` y **algún hallazgo** con severidad ≥ `fail_threshold`; `2` = fallo de la herramienta. `gate.sh` lo traduce en el resultado del job.

## 10. Estrategia de testing

- **Unitarios (Go)**: todos los paquetes de `internal/` y `cmd/`, con `testing` + `testify`. Los parsers usan fixtures JSON con la forma real de las salidas de gitleaks/trivy, con valores ficticios. Nunca se ejecutan los binarios reales: `internal/scanners` solo prueba "binario ausente", la regla de `isExecutionFailure` y el timeout (con el propio ejecutable de test como escáner falso).
- **Scripts de la Action (bash)**: `shellcheck` + `bats`, con `gh`/`curl`/`pipelineguard` falsos en PATH. Nunca red ni GitHub real.
- **Integración / end-to-end**: **no hay un test automatizado**. La validación end-to-end se hizo **a mano**, corriendo la Action real (release `v0.1.0`) contra un Pull Request real en `pipelineguard-demo-vulnerable-app`. Esa corrida encontró el bug `-f` vs `-F` de `comment.sh` (ver `docs/action-comment.md`). Automatizar esa prueba sigue pendiente.
- **CI del propio proyecto**: `ci.yml` corre build + `go vet` + `gofmt` + tests, `golangci-lint`, y `shellcheck` + `bats`, en cada PR y push a `main` (ver `docs/ci-cd.md`).

## 11. Release y versionado

- SemVer estricto (`v1.0.0`, `v1.2.3`).
- Tag flotante `v1` apuntando siempre al último release de la serie 1.x, para que los usuarios puedan fijar `uses: CamiloUrrea/PipelineGuard/action@v1`.
- Flujo completo automatizado en `.github/workflows/release.yml`: GoReleaser (build matrix multiplataforma → checksums → changelog → publicación del GitHub Release) y, solo si GoReleaser tuvo éxito, el step **`Update floating major version tag`**, que deriva el major del tag (`v1.0.0` → `v1`) y hace `git tag -fa` + `git push --force` de ese tag mayor sobre el commit recién publicado (ver `docs/ci-cd.md`). El tag mayor lo mueve ese step, no GoReleaser.
- Pre-releases (`v1.1.0-rc.1`, cualquier tag que no sea `vX.Y.Z` limpio) **no** mueven el tag flotante, y `install.sh` tampoco las elige al resolver `v1`.
- Hoy `checksums.txt` **no** va firmado. La firma de binarios con cosign es roadmap **v1.4**.

## 12. Roadmap resumido

| Versión | Contenido |
|---|---|
| v1.0 | gitleaks + trivy, modo informativo, comentario en PR + SARIF |
| v1.1 | integración con semgrep + aplicar `ignore_paths` |
| v1.2 | modo diff-only (solo hallazgos nuevos del PR) |
| v1.3 | soporte IaC (Checkov/tfsec) |
| v1.4 | firma de binarios con cosign |
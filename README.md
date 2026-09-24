# PipelineGuard

[![CI](https://github.com/CamiloUrrea/PipelineGuard/actions/workflows/ci.yml/badge.svg)](https://github.com/CamiloUrrea/PipelineGuard/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/CamiloUrrea/PipelineGuard)](https://github.com/CamiloUrrea/PipelineGuard/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**PipelineGuard** orquesta escáneres de seguridad ya probados en la industria ([gitleaks](https://github.com/gitleaks/gitleaks) para secretos, [trivy](https://github.com/aquasecurity/trivy) para vulnerabilidades de dependencias) y consolida sus hallazgos en **un solo reporte**, con severidad normalizada y un risk score agregado — en vez de revisar tres formatos de salida distintos en cada Pull Request.

Se instala en cualquier repositorio con una línea en tu workflow. Gratis, open source, sin cuenta ni backend externo.

## Qué hace

- Corre `gitleaks` y `trivy` sobre tu repositorio.
- Normaliza sus hallazgos a un esquema común con severidad unificada (`CRITICAL`/`HIGH`/`MEDIUM`/`LOW`/`INFO`).
- Publica un comentario en el Pull Request con la tabla de hallazgos y el risk score — y lo **actualiza** en cada push nuevo, sin duplicarlo.
- Sube un reporte [SARIF](https://docs.github.com/code-security/code-scanning/integrating-with-code-scanning/sarif-support-for-code-scanning) a la pestaña **Security** de GitHub.
- Por defecto es **informativo**: nunca bloquea tu PR. Tú decides si y cuándo activar el bloqueo real.

## Uso

Agrega esto a un workflow de tu repositorio (ej. `.github/workflows/security.yml`):

```yaml
name: Security scan

on:
  pull_request:

permissions:
  contents: read
  pull-requests: write
  security-events: write

jobs:
  pipelineguard:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7

      # gitleaks y trivy no vienen preinstalados en los runners — instálalos antes:
      - uses: actions/setup-go@v7
        with:
          go-version: 'stable'
      - run: go install github.com/zricethezav/gitleaks/v8@latest
      - run: curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin

      - name: Run PipelineGuard
        uses: CamiloUrrea/PipelineGuard/action@v1
```

Eso es todo — sin `.pipelineguard.yml`, corre con la configuración por defecto: ambos escáneres activos, modo informativo (nunca falla el build).

## Configuración (opcional)

Crea un `.pipelineguard.yml` en la raíz de tu repo para ajustar el comportamiento:

```yaml
scanners:
  gitleaks: true
  trivy: true

enforce: true              # si es true, falla el build al superar fail_threshold
fail_threshold: CRITICAL   # CRITICAL | HIGH | MEDIUM

severity_weights:
  CRITICAL: 10
  HIGH: 5
  MEDIUM: 2
  LOW: 1
```

Ver [`docs/config.md`](docs/config.md) para el esquema completo.

## Cómo interpretar el resultado

| Código de salida | Significado |
|---|---|
| `0` | Sin hallazgos que superen el umbral configurado. |
| `1` | Hallazgo(s) que igualan o superan `fail_threshold`, con `enforce: true`. |
| `2` | Fallo de la herramienta en sí (escáner no encontrado, timeout, error de configuración) — no es un veredicto de seguridad. |

## Documentación

Este README cubre lo esencial para usarlo. Para el diseño interno, decisiones de arquitectura y el detalle de cada componente:

- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — visión general y modelo de severidad.
- [`docs/README.md`](docs/README.md) — índice de todos los módulos, con explicación de cada uno.

## Estado del proyecto

`v1.0.0`. Funcional y validado de extremo a extremo contra un repositorio real (ver [pipelineguard-demo-vulnerable-app](https://github.com/CamiloUrrea/pipelineguard-demo-vulnerable-app)). Limitaciones conocidas, documentadas honestamente:

- `ignore_paths` se acepta en la configuración pero todavía no se aplica (reservado para v1.1).
- `semgrep` está en el roadmap (v1.1), no implementado todavía.
- Los checksums de los binarios garantizan integridad, no autenticidad (firma con cosign planeada para v1.4).

## Licencia

[MIT](LICENSE)
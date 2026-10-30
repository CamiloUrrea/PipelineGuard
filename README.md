# PipelineGuard

[![CI](https://github.com/CamiloUrrea/PipelineGuard/actions/workflows/ci.yml/badge.svg)](https://github.com/CamiloUrrea/PipelineGuard/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/CamiloUrrea/PipelineGuard)](https://github.com/CamiloUrrea/PipelineGuard/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**PipelineGuard** orquesta escáneres de seguridad ya probados en la industria ([gitleaks](https://github.com/gitleaks/gitleaks) para secretos, [trivy](https://github.com/aquasecurity/trivy) para vulnerabilidades de dependencias) y consolida sus hallazgos en **un solo reporte**, con severidad normalizada y un risk score agregado — en vez de revisar la salida de cada escáner por separado en cada Pull Request.

Se agrega a tu workflow con un step (`uses: CamiloUrrea/PipelineGuard/action@v1`), más los steps que instalan gitleaks y trivy, que no vienen en los runners (ver [Uso](#uso)). Gratis, open source, sin cuenta ni backend externo.

## Qué hace

- Corre `gitleaks` y `trivy` sobre tu repositorio.
- Normaliza sus hallazgos a un esquema común con severidad unificada (`CRITICAL`/`HIGH`/`MEDIUM`/`LOW`/`INFO`).
- Publica un comentario en el Pull Request con la tabla de hallazgos y el risk score — y lo **actualiza** en cada push nuevo, sin duplicarlo. Si hay más de 100 hallazgos, el comentario muestra los 100 más graves y avisa cuántos omitió; el detalle completo queda en la pestaña Security.
- Sube un reporte [SARIF](https://docs.github.com/code-security/code-scanning/integrating-with-code-scanning/sarif-support-for-code-scanning) a la pestaña **Security** de GitHub.
- Agrega el reporte al **resumen del job**, así que se ve aunque no se haya podido publicar el comentario (no existe en `v1.0.x`).
- Por defecto es **informativo**: los hallazgos no hacen fallar tu PR. Tú decides si y cuándo activar el bloqueo real. Ojo: un **fallo de la herramienta** (exit `2`: escáner ausente, timeout, configuración inválida) **sí** hace fallar el job también en modo informativo, para que un escaneo roto nunca pase por "0 hallazgos".
- El resultado del job lo decide **solo el gate** (el exit code de `pipelineguard`): si falla la subida del SARIF o la publicación del comentario (PRs desde forks, repos privados sin GitHub Advanced Security), la Action emite un aviso (`::warning`) y el job no se pone en rojo por eso. Con `v1.0.x` el comportamiento es el anterior: esos fallos sí hacen fallar el job (ver [Limitaciones conocidas](#limitaciones-conocidas)).

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
          cache: false
      - run: go install github.com/zricethezav/gitleaks/v8@v8.30.1
      - run: curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin v0.72.0

      - name: Run PipelineGuard
        uses: CamiloUrrea/PipelineGuard/action@v1
```

> **Nota sobre el instalador de trivy:** el script `install.sh` se descarga desde la rama `main` de trivy, no desde un release fijo (la versión `v0.72.0` sí queda fijada). Para máxima seguridad, descarga el binario del [release de trivy](https://github.com/aquasecurity/trivy/releases) y verifica su checksum.

Con eso, sin `.pipelineguard.yml`, corre con la configuración por defecto: ambos escáneres activos y modo informativo (los hallazgos no fallan el build).

### Qué versión del binario se instala

El binario **sigue a la versión de la Action**: con `@v1` se instala el último release `1.x`, y con un tag completo como `@v1.1.0`, exactamente ese tag. Si fijas la Action a una rama o a un SHA, indica la versión del binario con el input `version:` (p. ej. `version: v1` o `version: v1.1.0`); si no, la instalación falla con un mensaje que lo pide. Con `v1.0.x` el comportamiento es el anterior: la Action instala el último `1.x` aunque la fijes a `@v1.0.0` (ver [Limitaciones conocidas](#limitaciones-conocidas)).

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
| `0` | Sin hallazgos que igualen o superen `fail_threshold`, **o** modo informativo (`enforce: false`, el default) aunque haya hallazgos. |
| `1` | Hallazgo(s) que igualan o superan `fail_threshold`, con `enforce: true`. |
| `2` | Fallo de la herramienta en sí (escáner no encontrado, timeout, error de configuración) — no es un veredicto de seguridad. Hace fallar el job también en modo informativo. |

## Documentación

Este README cubre lo esencial para usarlo. Para el diseño interno, decisiones de arquitectura y el detalle de cada componente:

- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — visión general y modelo de severidad.
- [`docs/README.md`](docs/README.md) — índice de todos los módulos, con explicación de cada uno.
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — cómo correr los tests y proponer cambios.
- [`SECURITY.md`](SECURITY.md) — cómo reportar una vulnerabilidad de forma privada.

## Estado del proyecto

- **Versión estable publicada: `v1.1.0`.** El tag flotante `v1` apunta a ella.
- **Validado** en ejecuciones reales de GitHub Actions (runner `ubuntu-24.04`):
  - Con `@v1`, `install.sh` resuelve `v1` a `v1.1.0`, descarga `pipelineguard_1.1.0_linux_amd64.tar.gz` y verifica su checksum.
  - Con la Action fijada a un tag completo (p. ej. `@v1.1.0-rc.1`) y sin input `version:`, se instala el binario de ese tag (`github.action_ref`).
  - `continue-on-error` funciona en los steps `uses:` de la composite action: con la subida del SARIF fallando (ruta inexistente), los steps siguientes se ejecutan, aparece el aviso y el reporte queda en el resumen del job. El resultado lo decide el gate.
  - Sin `pull-requests: write`, el comentario falla con `403`, aparece el aviso y el job solo falla por el gate.
  - El escaneo del propio repo (`self-scan.yml`) y Dependabot funcionan.
- **No validado:** Windows y macOS, Ubuntu 26, PRs desde forks, repos privados sin GitHub Advanced Security, y repos con muchos hallazgos (el truncado del comentario solo tiene pruebas unitarias).

### Limitaciones conocidas

- **PRs desde forks:** GitHub les da un `GITHUB_TOKEN` de solo lectura, así que la publicación del comentario y la subida del SARIF pueden fallar. Eso solo genera un aviso y el reporte queda en el resumen del job; el job no falla por eso (sí si falla el gate). No se ha probado con un fork real. Con `v1.0.x` el comportamiento es el anterior: el job falla.
- **Repos privados sin GitHub Advanced Security:** la subida del SARIF (`upload-sarif`) puede fallar, porque code scanning no está disponible. Mismo tratamiento que en los forks: aviso y el gate decide; con `v1.0.x`, job en rojo. Tampoco se ha probado en un repo así.
- `semgrep` no está implementado ni en el roadmap: `scanners.semgrep` se sigue aceptando por compatibilidad, pero se ignora (con `true`, el binario lo avisa en stderr).
- `ignore_paths` se acepta en la configuración pero no se aplica: los hallazgos en esas rutas siguen apareciendo. Está pendiente, sin versión asignada.
- **Fijar la versión en `v1.0.x`:** `uses: CamiloUrrea/PipelineGuard/action@v1.0.0` fija los scripts de la Action, pero el binario se resuelve aparte con el input `version` (default `v1` en esas versiones), así que igual instala el último release `1.x`. Para fijar también el binario, agrega `with: { version: v1.0.0 }`. En `v1.1.0` el binario sigue a la versión de la Action (ver [Qué versión del binario se instala](#qué-versión-del-binario-se-instala)).
- **Windows y macOS:** se publican binarios para ambos, pero no se han probado en un runner real.
- **Ubuntu 26:** sin probar; las ejecuciones reales cayeron en runners `ubuntu-24.04`.
- **Orden de los hallazgos:** entre hallazgos de igual severidad, el orden puede variar de una ejecución a otra.
- Los checksums de los binarios garantizan integridad, no autenticidad (firma con cosign planeada para v1.4).

## Licencia

[MIT](LICENSE) — Camilo Urrea

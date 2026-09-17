# Documentación de PipelineGuard

PipelineGuard es una herramienta en Go que orquesta escáneres de seguridad
(gitleaks, trivy) y consolida sus hallazgos en un reporte unificado.

## Índice por bloques

- [Bloque 1] Modelo Finding + parser de gitleaks — ver docs/parsers.md
- [Bloque 2] Parser de trivy (vulnerabilidades de dependencias) — ver docs/parsers.md
- [Bloque 3] Cálculo de risk score — ver docs/scoring.md
- [Bloque 4] Generación de reporte Markdown — ver docs/report.md
- [Bloque 5] Generación de SARIF — ver docs/sarif.md
- [Bloque 6] Carga de configuración (.pipelineguard.yml) — ver docs/config.md
- [Bloque 7] Decisión de enforcement (ShouldFail) — ver docs/policy.md
- [Bloque 8] Orquestación end-to-end (internal/orchestrator) — ver docs/orchestrator.md
- [Bloque 9] Ejecución real de escáneres (internal/scanners) — ver docs/scanners.md
- [Bloque 10] Binario CLI (cmd/pipelineguard) — ver docs/cmd.md
- [Bloque 11] Configuración de release (.goreleaser.yaml) — ver docs/goreleaser.md
- [Bloque 12] Instalación verificada del binario (action/scripts/install.sh) — ver docs/action-install.md
- [Bloque 13] Ejecución del binario y composite action (action.yml) — ver docs/action-run.md y docs/action.md
- [Bloque 14] Subida de SARIF a GitHub Security — ver docs/action.md
- [Bloque 15] Publicación/actualización del comentario en el PR (action/scripts/comment.sh) — ver docs/action-comment.md
- [Bloque 16] Step final de enforcement (action/scripts/gate.sh) — ver docs/action-gate.md
- [Bloque 17] CI/CD del propio proyecto (.github/workflows/) — ver docs/ci-cd.md

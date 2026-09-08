# Documentación de PipelineGuard

PipelineGuard es una herramienta en Go que orquesta escáneres de seguridad
(gitleaks, trivy) y consolida sus hallazgos en un reporte unificado.

## Índice por bloques

- [Bloque 1] Modelo Finding + parser de gitleaks — ver docs/parsers.md
- [Bloque 2] Parser de trivy (vulnerabilidades de dependencias) — ver docs/parsers.md
- [Bloque 3] Cálculo de risk score — ver docs/scoring.md
- [Bloque 4] Generación de reporte Markdown — ver docs/report.md
- [Bloque 5] Generación de SARIF — ver docs/sarif.md

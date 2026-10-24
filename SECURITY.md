# Política de seguridad

## Versiones con soporte

Solo la **última versión publicada de la serie 1.x** recibe correcciones de seguridad. Si usas una versión anterior, actualiza antes de reportar: puede que el problema ya esté corregido.

| Versión | Soporte |
|---|---|
| Última 1.x | ✅ |
| Cualquier 1.x anterior | ❌ |
| 0.x | ❌ |

## Cómo reportar una vulnerabilidad

**No abras un issue público.** Usa el reporte privado de vulnerabilidades de GitHub:

👉 <https://github.com/CamiloUrrea/PipelineGuard/security/advisories/new>

Incluye, en la medida de lo posible:

- La versión afectada (el ref de la Action y, si lo pasas, el input `version:`).
- Qué componente está afectado (binario `pipelineguard` o un script de `action/scripts/`).
- Pasos para reproducirlo, o una prueba de concepto mínima.
- El impacto que ves (p. ej. ejecución de comandos, filtración del `GITHUB_TOKEN`, un binario no verificado que se ejecuta).
- No incluyas secretos reales: si un ejemplo los necesita, usa valores ficticios.

## Alcance

**Dentro del alcance:**

- El binario `pipelineguard` (el código Go de `cmd/` e `internal/`).
- Los scripts y la definición de la composite action (`action/action.yml` y `action/scripts/`), incluida la descarga y verificación del binario.
- Los workflows de release del propio repositorio, en lo que afecte a los binarios publicados.

**Fuera del alcance:**

- Vulnerabilidades o falsos negativos de **gitleaks** o **trivy**: repórtalos a sus propios proyectos ([gitleaks](https://github.com/gitleaks/gitleaks), [trivy](https://github.com/aquasecurity/trivy)). PipelineGuard solo los ejecuta y consolida su salida.
- Los hallazgos que PipelineGuard encuentra en tu repositorio: son el resultado esperado de la herramienta, no una vulnerabilidad de PipelineGuard.
- Limitaciones ya documentadas en el [README](README.md#limitaciones-conocidas) (por ejemplo, que `checksums.txt` no va firmado).

## Qué esperar

PipelineGuard es un proyecto de **un solo mantenedor (Camilo Urrea)**. Los reportes se atienden con el **mejor esfuerzo**, sin plazos garantizados. La conversación sigue en el propio aviso privado, y la corrección sale en una nueva versión 1.x.

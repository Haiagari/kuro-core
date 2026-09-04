#!/usr/bin/env bash
set -euo pipefail

# Kuro Pre-commit Hook
# Previene commitear secretos usando Gitleaks.
#
# Instalación:
#   bash scripts/setup-hooks.sh
#
# Nota: Escanea archivos staged usando git diff para filtrar solo cambios.

printf '%s\n' "[kuro] 🔒 Running pre-commit security check..."

# Verificar que gitleaks esté instalado
if ! command -v gitleaks >/dev/null 2>&1; then
    printf '%s\n' "[warn] ⚠️  gitleaks not found. Install: https://github.com/gitleaks/gitleaks"
    printf '%s\n' "[warn]    Skipping local security check."
    exit 0
fi

# Determinar config path
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CONFIG="$PROJECT_ROOT/deploy/security/gitleaks/gitleaks.toml"

# Si el script está en .git/hooks/, buscar config en el proyecto
if [[ ! -f "$CONFIG" ]]; then
    CONFIG="$(git rev-parse --show-toplevel)/deploy/security/gitleaks/gitleaks.toml"
fi

# Obtener archivos staged (excluyendo .env, node_modules, etc.)
STAGED_FILES=$(git diff --cached --name-only --diff-filter=ACM | \
    grep -v '\.env$' | \
    grep -v '\.env\.' | \
    grep -v 'node_modules/' | \
    grep -v '\.opencode/' | \
    grep -v 'app-demo/vulnerable/' | \
    grep -v '\.git/' || true)

if [[ -z "$STAGED_FILES" ]]; then
    printf '%s\n' "✅ [OK] No scannable files staged."
    exit 0
fi

# Crear directorio temporal para archivos staged
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

# Copiar archivos staged a directorio temporal
echo "$STAGED_FILES" | while IFS= read -r file; do
    if [[ -f "$file" ]]; then
        mkdir -p "$TEMP_DIR/$(dirname "$file")"
        cp "$file" "$TEMP_DIR/$file" 2>/dev/null || true
    fi
done

# Correr gitleaks solo sobre archivos staged
if [[ -f "$CONFIG" ]]; then
    if ! gitleaks detect --config "$CONFIG" --no-git --source "$TEMP_DIR" --verbose 2>&1 | grep -v "app-demo/"; then
        printf '\n%s\n' "❌ [BLOCKED] Secrets detected in staged files!"
        printf '%s\n' "   Fix the secrets or request an exception before committing."
        printf '%s\n' "   To bypass (emergency only): git commit --no-verify"
        exit 1
    fi
else
    # Fallback: usar config por defecto de gitleaks
    if ! gitleaks detect --no-git --source "$TEMP_DIR" --verbose 2>&1 | grep -v "app-demo/"; then
        printf '\n%s\n' "❌ [BLOCKED] Secrets detected in staged files!"
        printf '%s\n' "   Fix the secrets or request an exception before committing."
        exit 1
    fi
fi

printf '%s\n' "✅ [OK] No secrets detected in staged files."
exit 0

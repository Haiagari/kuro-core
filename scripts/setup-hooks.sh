#!/usr/bin/env bash
set -euo pipefail

# Kuro — Setup Pre-commit Hook
# Instala el pre-commit hook de gitleaks en el repositorio local.
#
# Uso:
#   bash scripts/setup-hooks.sh
#
# Esto copia el script de pre-commit a .git/hooks/pre-commit
# y lo hace ejecutable.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
HOOK_SOURCE="$SCRIPT_DIR/pre-commit-secrets.sh"
HOOK_DEST="$PROJECT_ROOT/.git/hooks/pre-commit"

echo "🔧 Setting up pre-commit hooks..."

# Verificar que es un repositorio git
if [[ ! -d "$PROJECT_ROOT/.git" ]]; then
    echo "❌ Error: Not a git repository"
    exit 1
fi

# Verificar que gitleaks esté instalado
if ! command -v gitleaks >/dev/null 2>&1; then
    echo "⚠️  Warning: gitleaks not found"
    echo "   Install: https://github.com/gitleaks/gitleaks"
    echo "   Or: brew install gitleaks (macOS)"
    echo "   Or: go install github.com/gitleaks/gitleaks/v8@latest"
    echo ""
    echo "   The hook will still run but skip the security check."
fi

# Copiar hook
cp "$HOOK_SOURCE" "$HOOK_DEST"
chmod +x "$HOOK_DEST"

echo "✅ Pre-commit hook installed: $HOOK_DEST"
echo ""
echo "Now every 'git commit' will automatically scan for secrets."
echo "To bypass (emergency only): git commit --no-verify"

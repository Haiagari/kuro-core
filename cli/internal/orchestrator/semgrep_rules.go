package orchestrator

import _ "embed"

//go:embed rules/semgrep-core.yml
var semgrepCoreRules []byte

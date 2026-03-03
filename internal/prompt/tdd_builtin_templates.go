package prompt

import _ "embed"

//go:embed templates/PROMPT_tdd_red.md
var builtinTDDRedTemplate string

//go:embed templates/PROMPT_tdd_green.md
var builtinTDDGreenTemplate string

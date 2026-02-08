package skills

import _ "embed"

//go:embed gromit-refine/SKILL.md
var RefineSkill string

//go:embed gromit-plan/SKILL.md
var PlanSkill string

//go:embed gromit-decompose/SKILL.md
var DecomposeSkill string

//go:embed gromit-orchestrator/SKILL.md
var OrchestratorSkill string

//go:embed gromit-orchestrator/pipeline-resume.sh
var PipelineResumeHook string

//go:embed gromit-debug/SKILL.md
var DebugSkill string

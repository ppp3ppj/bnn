package ast

type StepKind string

const (
	StepPre  StepKind = "pre"
	StepRun  StepKind = "run"
	StepPost StepKind = "post"
)

type RuntimeKind string

const (
	RuntimeMise  RuntimeKind = "mise"
	RuntimeBrew  RuntimeKind = "brew"
	RuntimeShell RuntimeKind = "shell"
)

type StepNode struct {
	Kind    StepKind
	Command string
}

type RuntimeNode struct {
	Type    RuntimeKind
	Version string // empty when no version given (e.g. runtime(shell))
}

type BunchNode struct {
	Name    string
	Runtime RuntimeNode
	Depends []string
	Check   string // empty when no check guard
	Steps   []StepNode
}

type ManifestNode struct {
	Bunches []BunchNode
}

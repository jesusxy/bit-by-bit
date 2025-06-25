package runner

// Runner will orchestrate namespaces/cgroups later
type Runner struct{}

func New() *Runner {
	return &Runner{}
}

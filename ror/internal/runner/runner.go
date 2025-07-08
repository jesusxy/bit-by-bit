package runner

// Runner will orchestrate namespaces/cgroups later
type Runner struct {
	BasePath string
}

func New() *Runner {
	return &Runner{}
}

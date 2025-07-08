package runner

// Runner will orchestrate namespaces/cgroups later
type Runner struct {
	BasePath string
}

func New(basepath string) (*Runner, error) {
	return &Runner{
		BasePath: basepath,
	}, nil
}

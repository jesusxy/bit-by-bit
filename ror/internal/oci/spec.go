package oci

type Spec struct {
	Root struct {
		Path string `json:"path"`
	} `json:"root"`
	Process struct {
		Args []string `json:"args"`
		Cwd  string   `json:"cwd,omitempty"`
	} `json:"process"`
}

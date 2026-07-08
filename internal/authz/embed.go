package authz

import _ "embed"

// Model dan Policy di-embed dari file — di-load saat startup di main.go via
// authz.New(authz.Model, authz.Policy). Versionable di git.
var (
	//go:embed model.conf
	Model string

	//go:embed policy.csv
	Policy string
)

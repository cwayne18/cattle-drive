package api

// StatusRequest is the body for the status endpoint.
type StatusRequest struct {
	Kubeconfig          string `json:"kubeconfig"`
	Source              string `json:"source"`
	Target              string `json:"target"`
	TargetRancherConfig string `json:"targetRancherConfig,omitempty"`
}

// MigrateRequest is the body for the migrate endpoint.
type MigrateRequest struct {
	Kubeconfig          string `json:"kubeconfig"`
	Source              string `json:"source"`
	Target              string `json:"target"`
	TargetRancherConfig string `json:"targetRancherConfig,omitempty"`
}

// ObjectStatus represents the migration status of a single migratable object.
type ObjectStatus struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Migrated bool   `json:"migrated"`
	Diff     bool   `json:"diff"`
	// Description is a human-readable string for role bindings.
	Description string `json:"description,omitempty"`
}

// ProjectStatus extends ObjectStatus with nested PRTBs and namespaces.
type ProjectStatus struct {
	ObjectStatus
	PRTBs      []ObjectStatus `json:"prtbs"`
	Namespaces []ObjectStatus `json:"namespaces"`
}

// StatusResponse is the response body for GET /status.
type StatusResponse struct {
	Source       string          `json:"source"`
	Target       string          `json:"target"`
	Projects     []ProjectStatus `json:"projects"`
	CRTBs        []ObjectStatus  `json:"clusterRoleBindings"`
	ClusterRepos []ObjectStatus  `json:"catalogRepos"`
}

// MigrateLogEntry represents a single line in the migration output.
type MigrateLogEntry struct {
	Message string `json:"message"`
	Error   bool   `json:"error,omitempty"`
}

// MigrateResponse is the response body for POST /migrate.
type MigrateResponse struct {
	Source  string            `json:"source"`
	Target  string            `json:"target"`
	Log     []MigrateLogEntry `json:"log"`
	Success bool              `json:"success"`
	Error   string            `json:"error,omitempty"`
}

// ClusterInfo is the response item for GET /clusters.
type ClusterInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

// ClustersResponse is the response body for GET /clusters.
type ClustersResponse struct {
	Clusters []ClusterInfo `json:"clusters"`
}

// ErrorResponse is a generic error envelope.
type ErrorResponse struct {
	Error string `json:"error"`
}

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"rancherlabs/cattle-drive/pkg/client"
	"rancherlabs/cattle-drive/pkg/cluster"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// writeJSON serialises v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error envelope.
func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, ErrorResponse{Error: msg})
}

// handleClusters lists all non-local downstream clusters visible through the
// kubeconfig passed as a JSON body.
func handleClusters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req StatusRequest // reuse – only kubeconfig is needed
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Kubeconfig == "" {
		writeError(w, http.StatusBadRequest, "kubeconfig is required")
		return
	}

	ctx := context.Background()
	restCfg, err := clientcmd.BuildConfigFromFlags("", req.Kubeconfig)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to load kubeconfig: "+err.Error())
		return
	}
	cl, err := client.New(ctx, restCfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create client: "+err.Error())
		return
	}

	var list v3.ClusterList
	if err := cl.Clusters.List(ctx, "", &list, v1.ListOptions{}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list clusters: "+err.Error())
		return
	}

	// Use an initialised (non-nil) slice so the response marshals as [] not null.
	infos := []ClusterInfo{}
	for _, c := range list.Items {
		// skip local management cluster
		if c.Name == "local" {
			continue
		}
		infos = append(infos, ClusterInfo{
			ID:          c.Name,
			DisplayName: c.Spec.DisplayName,
		})
	}
	writeJSON(w, http.StatusOK, ClustersResponse{Clusters: infos})
}

// handleStatus compares source and target cluster objects and returns their
// migration status.
func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req StatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Kubeconfig == "" || req.Source == "" || req.Target == "" {
		writeError(w, http.StatusBadRequest, "kubeconfig, source and target are required")
		return
	}

	ctx := context.Background()
	sc, tc, cl, err := buildClusters(ctx, req.Kubeconfig, req.TargetRancherConfig, req.Source, req.Target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := sc.Populate(ctx, cl.source); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to populate source cluster: "+err.Error())
		return
	}
	if err := tc.Populate(ctx, cl.target); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to populate target cluster: "+err.Error())
		return
	}
	if err := sc.Compare(ctx, tc); err != nil {
		writeError(w, http.StatusInternalServerError, "comparison failed: "+err.Error())
		return
	}

	resp := buildStatusResponse(sc, tc)
	writeJSON(w, http.StatusOK, resp)
}

// handleMigrate runs the full migration and returns a structured log.
func handleMigrate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req MigrateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Kubeconfig == "" || req.Source == "" || req.Target == "" {
		writeError(w, http.StatusBadRequest, "kubeconfig, source and target are required")
		return
	}

	ctx := context.Background()
	sc, tc, cl, err := buildClusters(ctx, req.Kubeconfig, req.TargetRancherConfig, req.Source, req.Target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := sc.Populate(ctx, cl.source); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to populate source cluster: "+err.Error())
		return
	}
	if err := tc.Populate(ctx, cl.target); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to populate target cluster: "+err.Error())
		return
	}
	if err := sc.Compare(ctx, tc); err != nil {
		writeError(w, http.StatusInternalServerError, "comparison failed: "+err.Error())
		return
	}

	var buf bytes.Buffer
	migrateClient := cl.target
	migrateErr := sc.Migrate(ctx, migrateClient, tc, &buf)

	logEntries := parseLog(buf.String())
	// If the migration ended with an error, append it as a final error log entry
	// so the UI can display and count it alongside the progress lines.
	if migrateErr != nil {
		logEntries = append(logEntries, MigrateLogEntry{
			Message: "Error: " + migrateErr.Error(),
			Error:   true,
		})
	}
	// Ensure the log field never marshals as null.
	if logEntries == nil {
		logEntries = []MigrateLogEntry{}
	}
	resp := MigrateResponse{
		Source:  req.Source,
		Target:  req.Target,
		Log:     logEntries,
		Success: migrateErr == nil,
	}
	if migrateErr != nil {
		resp.Error = migrateErr.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}

// clusterClients holds the source and target API clients.
type clusterClients struct {
	source *client.Clients
	target *client.Clients
}

// newClientsFromReq builds source and (optionally separate) target Clients from
// the kubeconfig paths supplied in the request.
func newClientsFromReq(ctx context.Context, kubeconfigPath, targetKubeconfigPath string) (*clusterClients, error) {
	restCfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}
	sourceCl, err := client.New(ctx, restCfg)
	if err != nil {
		return nil, err
	}

	targetCl := sourceCl
	if targetKubeconfigPath != "" {
		targetRestCfg, err := clientcmd.BuildConfigFromFlags("", targetKubeconfigPath)
		if err != nil {
			return nil, err
		}
		targetCl, err = client.New(ctx, targetRestCfg)
		if err != nil {
			return nil, err
		}
	}
	return &clusterClients{source: sourceCl, target: targetCl}, nil
}

// buildClusters constructs and populates the source and target Cluster objects
// used for status/migrate operations.
func buildClusters(ctx context.Context, kubeconfigPath, targetKubeconfigPath, sourceName, targetName string) (*cluster.Cluster, *cluster.Cluster, *clusterClients, error) {
	cl, err := newClientsFromReq(ctx, kubeconfigPath, targetKubeconfigPath)
	if err != nil {
		return nil, nil, nil, err
	}

	restCfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, nil, nil, err
	}

	var targetRestCfg *rest.Config
	if targetKubeconfigPath != "" {
		targetRestCfg, err = clientcmd.BuildConfigFromFlags("", targetKubeconfigPath)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	var clusterList v3.ClusterList
	if err := cl.source.Clusters.List(ctx, "", &clusterList, v1.ListOptions{}); err != nil {
		return nil, nil, nil, err
	}

	var sourceObj, targetObj *v3.Cluster
	for _, c := range clusterList.Items {
		if c.Spec.DisplayName == sourceName {
			sourceObj = c.DeepCopy()
		}
		if targetKubeconfigPath == "" && c.Spec.DisplayName == targetName {
			targetObj = c.DeepCopy()
		}
	}

	if targetKubeconfigPath != "" {
		var targetClusterList v3.ClusterList
		if err := cl.target.Clusters.List(ctx, "", &targetClusterList, v1.ListOptions{}); err != nil {
			return nil, nil, nil, err
		}
		for _, c := range targetClusterList.Items {
			if c.Spec.DisplayName == targetName {
				targetObj = c.DeepCopy()
			}
		}
	}

	if sourceObj == nil || targetObj == nil {
		return nil, nil, nil, &clusterNotFoundError{source: sourceName, target: targetName, sourceFound: sourceObj != nil}
	}

	scCfg := *restCfg
	scCfg.Host = restCfg.Host + "/k8s/clusters/" + sourceObj.Name
	scClient, err := client.New(ctx, &scCfg)
	if err != nil {
		return nil, nil, nil, err
	}
	sc := &cluster.Cluster{Obj: sourceObj, Client: scClient}

	tcBaseCfg := restCfg
	if targetRestCfg != nil {
		tcBaseCfg = targetRestCfg
	}
	tcCfg := *tcBaseCfg
	tcCfg.Host = tcBaseCfg.Host + "/k8s/clusters/" + targetObj.Name
	tcClient, err := client.New(ctx, &tcCfg)
	if err != nil {
		return nil, nil, nil, err
	}
	tc := &cluster.Cluster{Obj: targetObj, Client: tcClient}

	if targetKubeconfigPath != "" {
		sc.ExternalRancher = true
		tc.ExternalRancher = true
	}

	return sc, tc, cl, nil
}

// buildStatusResponse converts a populated + compared source Cluster (sc) and its
// target Cluster (tc) into a StatusResponse suitable for JSON serialisation.
func buildStatusResponse(sc, tc *cluster.Cluster) StatusResponse {
	resp := StatusResponse{
		Source:       sc.Obj.Spec.DisplayName,
		Target:       tc.Obj.Spec.DisplayName,
		// Initialise slices so they marshal as [] rather than null.
		Projects:     []ProjectStatus{},
		CRTBs:        []ObjectStatus{},
		ClusterRepos: []ObjectStatus{},
	}

	for _, p := range sc.ToMigrate.Projects {
		ps := ProjectStatus{
			ObjectStatus: ObjectStatus{
				Name:     p.Name,
				Type:     "project",
				Migrated: p.Migrated,
				Diff:     p.Diff,
			},
			PRTBs:      []ObjectStatus{},
			Namespaces: []ObjectStatus{},
		}
		for _, prtb := range p.PRTBs {
			ps.PRTBs = append(ps.PRTBs, ObjectStatus{
				Name:        prtb.Name,
				Type:        "prtb",
				Migrated:    prtb.Migrated,
				Diff:        prtb.Diff,
				Description: prtb.Description,
			})
		}
		for _, ns := range p.Namespaces {
			ps.Namespaces = append(ps.Namespaces, ObjectStatus{
				Name:     ns.Name,
				Type:     "namespace",
				Migrated: ns.Migrated,
				Diff:     ns.Diff,
			})
		}
		resp.Projects = append(resp.Projects, ps)
	}

	for _, crtb := range sc.ToMigrate.CRTBs {
		resp.CRTBs = append(resp.CRTBs, ObjectStatus{
			Name:        crtb.Name,
			Type:        "crtb",
			Migrated:    crtb.Migrated,
			Diff:        crtb.Diff,
			Description: crtb.Description,
		})
	}

	for _, repo := range sc.ToMigrate.ClusterRepos {
		resp.ClusterRepos = append(resp.ClusterRepos, ObjectStatus{
			Name:     repo.Name,
			Type:     "clusterrepo",
			Migrated: repo.Migrated,
			Diff:     repo.Diff,
		})
	}

	return resp
}

// parseLog converts the text output of Migrate() into structured log entries.
func parseLog(output string) []MigrateLogEntry {
	var entries []MigrateLogEntry
	for _, line := range splitLines(output) {
		if line == "" {
			continue
		}
		entries = append(entries, MigrateLogEntry{
			Message: line,
			Error:   false,
		})
	}
	return entries
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// clusterNotFoundError is returned when one or both clusters cannot be found.
type clusterNotFoundError struct {
	source, target string
	sourceFound    bool
}

func (e *clusterNotFoundError) Error() string {
	if !e.sourceFound {
		return "source cluster '" + e.source + "' not found"
	}
	return "target cluster '" + e.target + "' not found"
}

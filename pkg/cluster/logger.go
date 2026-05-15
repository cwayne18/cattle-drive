package cluster

import (
	"fmt"
	"io"
)

// MigrateEvent describes the outcome of migrating a single object.
// Err is nil when the operation succeeded.
type MigrateEvent struct {
	Kind string // "user", "project", "prtb", "namespace", "crtb", "clusterrepo"
	Name string
	Err  error
}

// MigrateLogger receives structured events emitted by Cluster.Migrate, one per
// migrated object. Using an interface lets the CLI format events as text while
// the HTTP API collects them as typed structs without any string parsing.
type MigrateLogger interface {
	LogEvent(MigrateEvent)
}

// WriterLogger formats events as human-readable text lines and writes them to W.
// It is used by the CLI and TUI so their output is unchanged.
type WriterLogger struct {
	W io.Writer
}

func (l *WriterLogger) LogEvent(e MigrateEvent) {
	indent := ""
	if e.Kind == "prtb" || e.Kind == "namespace" {
		indent = "  "
	}
	if e.Err != nil {
		fmt.Fprintf(l.W, "%s- migrating %s [%s]... Error: %v\n", indent, e.Kind, e.Name, e.Err)
	} else {
		fmt.Fprintf(l.W, "%s- migrating %s [%s]... Done.\n", indent, e.Kind, e.Name)
	}
}

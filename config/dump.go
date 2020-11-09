package config

import "k8s.io/apimachinery/pkg/runtime/schema"

// DumpCommand is the configuration for the dump subcommand
type DumpCommand struct {
	Dumps []Dump
}

// Dump defines the configuration fields for a single gvr dump
// GVR represents a k8s GroupVersionResource
type Dump struct {
	GVR       schema.GroupVersionResource
	Namespace string
	Filters   Filter
}

// Filter defines the configuration for a dump filter
type Filter struct {
	Ands []FilterElement
	Ors  []FilterElement
}

// FilterElement is the key value to filter for
type FilterElement struct {
	Key   string
	Value string
}

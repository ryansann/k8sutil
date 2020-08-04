package config

import "k8s.io/apimachinery/pkg/runtime/schema"

// Root is the configuration for the root k8sdump command
type Root struct {
	KubeConfig string
	Dumps      []Dump
}

// GVR represents a k8s GroupVersionResource
type Dump struct {
	GVR       schema.GroupVersionResource
	Namespace string
	Filters   Filter
}

type Filter struct {
	Ands []FilterElement
	Ors  []FilterElement
}

type FilterElement struct {
	Key   string
	Value string
}

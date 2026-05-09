package sidecar

// ExtractionMode represents the current document extraction backend mode.
type ExtractionMode string

const (
	ExtractionDetecting      ExtractionMode = "detecting"
	ExtractionJavaPrimary    ExtractionMode = "java_primary"
	ExtractionPythonFallback ExtractionMode = "python_fallback_degraded"
	ExtractionUnavailable    ExtractionMode = "unavailable"
)

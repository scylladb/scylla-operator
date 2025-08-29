# API design conventions

This document outlines the guidelines for designing our CRD APIs. The goal is to ensure consistency, usability, and maintainability across all our APIs.

As a base for these guidelines, we follow the [Kubernetes API conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
document. If something is not covered there, we codify our own conventions here.

## Tagged (a.k.a. "discriminated") union

Use this convention whenever a field's configuration options depends on a "type" field chosen by a user, and
the options are mutually exclusive.

- The "type" field should be named `Type`, and be of a string type (usually a custom string type with constants).
- The configuration options for a given type should be grouped in a struct with a name suffixed with `<Type>Options`.
- The configuration options fields' names should be suffixed with `<Type>Options` where `<Type>` is the value of the type field.
- The configuration options fields should be pointers and marked as `+optional` in the comments, to allow them to be omitted when not used.
- The configuration option fields should be marked with `omitempty` in the JSON tags, to avoid cluttering the serialized output.
- The validating server should validate that the configuration options corresponding to the chosen type are provided.

**Example:**
```go
type ExampleType string

const (
    ExampleTypeA ExampleType = "TypeA"
    ExampleTypeB ExampleType = "TypeB"
)

// ExampleSpec demonstrates a tagged (discriminated) union.
// Exactly one of {TypeAOptions, TypeBOptions} must be set, and it must match Type.
type ExampleSpec struct {
    // Type of the example. Determines which configuration options are used.
    Type ExampleType `json:"type"`

    // Configuration options for TypeA.
    // +optional
    TypeAOptions *TypeAOptions `json:"typeAOptions,omitempty"`

    // Configuration options for TypeB.
    // +optional
    TypeBOptions *TypeBOptions `json:"typeBOptions,omitempty"`
}
```

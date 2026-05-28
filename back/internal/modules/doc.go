// Package modules wires the modular monolith business modules together.
//
// Each module directory is organized with a consistent lightweight structure:
//   - provider.go: dependency construction for the module
//   - http.go: HTTP route registration
//   - consumer.go: async consumer registration
//
// This keeps assembly logic close to the module while allowing the underlying
// controller/service/repository code to migrate incrementally later.
package modules

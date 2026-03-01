// Package hmi provides the web-based Human Machine Interface for the plant
// simulator. It serves two distinct operator views:
//
//   - Water treatment HMI (port 8080): modern operator interface reflecting
//     the Purdue Level 2 operator station, with real-time process values
//     polled from the Level 1 PLCs.
//
//   - Manufacturing HMI (port 8081): legacy operator interface styled after
//     Wonderware InTouch, representing the Windows XP workstation on the
//     manufacturing flat network.
//
// The HMI uses chi v5 for HTTP routing, Bootstrap 5 for layout, and HTMX
// for real-time register value updates. Static assets are embedded via
// go:embed for single-binary deployment.
//
// Implemented in SOW-003.0 (water treatment) and SOW-004.0 (manufacturing).
package hmi

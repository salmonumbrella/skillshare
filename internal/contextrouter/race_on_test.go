//go:build race

package contextrouter

// raceBuild is true under the race detector, where the scanners run some fifty times slower.
const raceBuild = true

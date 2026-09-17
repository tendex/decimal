// Package benchmarks compares the speed of github.com/tendex/decimal with
// float64 and with other Go decimal libraries. It is a separate module so
// that the decimal module itself keeps no dependencies. It contains only
// tests, benchmarks and the command that turns their results into the tables
// in the README; see README.md.
package benchmarks

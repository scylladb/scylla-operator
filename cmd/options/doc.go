package options

// Package options is meant to be used along with cobra to provide
// an easy way to set flags and then access them from anywhere in the code.
//
// The implementation is not thread-safe, as it is assumed that flags
// won't change after the initial user input.
//
// It uses the singleton pattern.
//
// Example:
// var cmd = &cobra.Command{Run: run}
// options.GetCommonOptions().AddFlags(cmd)
// ...
// func run(cmd *cobra.Command, args []string) {
//   err := options.GetCommonOptions().Validate()
//   if err != nil {
//     ...
//   }
//

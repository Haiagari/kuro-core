package cmd


import "kuro/cli/internal/doctor"

// RunDoctor handles the `kuro doctor [--json]` command.
func RunDoctor(args []string) {
	doctor.RunDoctor(args)
}

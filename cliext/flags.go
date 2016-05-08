package cliext

import (
	"flag"
	"fmt"
)

// FlagsFlag is a github.com/codegangsta/cli.Flag based on a golang flag.Flag.
type FlagsFlag struct {
	*flag.Flag
	Hidden bool
}

// Apply adds a FlagsFlag to a flag.FlagSet.
func (f FlagsFlag) Apply(fs *flag.FlagSet) {
	fs.Var(f.Flag.Value, f.Name, f.Usage)
}

// GetName returns the FlagsFlag name.
func (f FlagsFlag) GetName() string {
	return f.Name
}

// String converts a FlagsFlag into a string used for Usage help.
func (f FlagsFlag) String() string {
	return fmt.Sprintf("--%s=%v\t%v", f.Flag.Name, f.Flag.Value, f.Flag.Usage)
}

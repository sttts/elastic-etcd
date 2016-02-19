package cliext

import (
	"flag"
	"fmt"
)

type FlagsFlag struct {
	*flag.Flag
}

func (f FlagsFlag) Apply(flag *flag.FlagSet) {
	flag.Var(f.Flag.Value, f.Name, f.Usage)
}

func (f FlagsFlag) GetName() string {
	return f.Name
}

func (f FlagsFlag) String() string {
	return fmt.Sprintf("--%s=%v\t%v", f.Flag.Name, f.Flag.Value, f.Flag.Usage)
}

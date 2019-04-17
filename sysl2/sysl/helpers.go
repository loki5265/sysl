package main

import (
	"strings"

	"github.com/anz-bank/sysl/src/proto"
	"github.com/sirupsen/logrus"
)

type arrayFlags []string

// String implements flag.Value.
func (i *arrayFlags) String() string {
	return strings.Join(*i, ",")
}

// Set implements flag.Value.
func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func getAppName(appname *sysl.AppName) string {
	return strings.Join(appname.Part, " :: ")
}

func getApp(appName *sysl.AppName, mod *sysl.Module) *sysl.Application {
	return mod.Apps[getAppName(appName)]
}

func hasAbstractPattern(attrs map[string]*sysl.Attribute) bool {
	patterns, has := attrs["patterns"]
	if has {
		if x := patterns.GetA(); x != nil {
			for _, y := range x.Elt {
				if y.GetS() == "abstract" {
					return true
				}
			}
		}
	}
	return false
}

func isSameApp(a *sysl.AppName, b *sysl.AppName) bool {
	if len(a.Part) != len(b.Part) {
		return false
	}
	for i := range a.Part {
		if a.Part[i] != b.Part[i] {
			return false
		}
	}
	return true
}

func isSameCall(a *sysl.Call, b *sysl.Call) bool {
	return isSameApp(a.Target, b.Target) && a.Endpoint == b.Endpoint
}

func loadApp(root string, models string) *sysl.Module {
	// Model we want to generate seqs for, the non-empty model
	mod, err := Parse(models, root)
	if err == nil {
		return mod
	}
	logrus.Errorf("unable to load module:\n\troot: " + root + "\n\tmodel:" + models)
	return nil
}

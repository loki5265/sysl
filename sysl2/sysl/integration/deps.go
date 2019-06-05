package integration

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/anz-bank/sysl/src/proto"
	"github.com/anz-bank/sysl/sysl2/sysl/utils"
	"github.com/yale8848/stream"
)

type AppElement struct {
	Name     string
	Endpoint string
}

type AppDependency struct {
	Self   *AppElement
	Target *AppElement
}

type DependencySet struct {
	Deps   map[*AppDependency]struct{}
	errors []string
}

func (dep *AppDependency) String() string {
	return fmt.Sprintf("s.name: %s, s.end: %s, t.name: %s, t.end: %s", dep.Self.Name, dep.Self.Endpoint, dep.Target.Name, dep.Target.Endpoint)
}

func (dep *AppDependency) extractAppNames() utils.StrSet {
	s := utils.StrSet{}
	s.Insert(dep.Self.Name)
	s.Insert(dep.Target.Name)

	return s
}

func (dep *AppDependency) extractEndpoints() utils.StrSet {
	s := utils.StrSet{}
	s.Insert(dep.Self.Endpoint)
	s.Insert(dep.Target.Endpoint)

	return s
}

func NewDependencySet() *DependencySet {
	return &DependencySet{map[*AppDependency]struct{}{}, []string{}}
}

func MakeAppDependency(self, target *AppElement) *AppDependency {
	return &AppDependency{self, target}
}

func MakeAppElement(name, endpoint string) *AppElement {
	return &AppElement{name, endpoint}
}

type CallSlice struct {
	slice []*sysl.Call
}

func NewCallSlice() *CallSlice {
	return &CallSlice{}
}

func (cs *CallSlice) Add(call *sysl.Call) {
	cs.slice = append(cs.slice, call)
}

func (cs *CallSlice) GetSlice() []*sysl.Call {
	return cs.slice
}

func (cs *CallSlice) CollectCallStatements(stmts []*sysl.Statement) {
	for _, stat := range stmts {
		switch c := stat.GetStmt().(type) {
		case *sysl.Statement_Call:
			cs.Add(c.Call)
		case *sysl.Statement_Action, *sysl.Statement_Ret:
			continue
		case *sysl.Statement_Cond:
			cs.CollectCallStatements(c.Cond.GetStmt())
		case *sysl.Statement_Loop:
			cs.CollectCallStatements(c.Loop.GetStmt())
		case *sysl.Statement_LoopN:
			cs.CollectCallStatements(c.LoopN.GetStmt())
		case *sysl.Statement_Foreach:
			cs.CollectCallStatements(c.Foreach.GetStmt())
		case *sysl.Statement_Group:
			cs.CollectCallStatements(c.Group.GetStmt())
		case *sysl.Statement_Alt:
			for _, choice := range c.Alt.GetChoice() {
				cs.CollectCallStatements(choice.GetStmt())
			}
		default:
			panic("No statement!")
		}
	}
}

func FindApps(module *sysl.Module, excludes, integrations utils.StrSet, ds *DependencySet, highlight bool) utils.StrSet {
	r := []string{}
	appReStr := toPattern(integrations.ToSlice())
	re := regexp.MustCompile(appReStr)
	st, _ := stream.Of(ds.buildArray())
	st.ForEach(func(v stream.T) bool {
		if dep, ok := v.(*AppDependency); ok {
			appNames := dep.extractAppNames()
			excludeApps := appNames.Intersection(excludes)
			if len(excludeApps) > 0 {
				return true
			}
			highlightApps := appNames.Intersection(integrations)
			if !highlight && len(highlightApps) == 0 {
				return true
			}
			appStream, _ := stream.Of(appNames.ToSlice())
			appStream.ForEach(func(v stream.T) bool {
				item := v.(string)
				app := module.GetApps()[item]
				if highlight {
					if re.MatchString(item) &&
						!hasPattern("human", utils.MakeStrSetFromSpecificAttr("patterns", app.GetAttrs()).ToSlice()) {
						r = append(r, item)
					}
					return true
				}
				if !hasPattern("human", utils.MakeStrSetFromSpecificAttr("patterns", app.GetAttrs()).ToSlice()) {
					r = append(r, item)
				}
				return true
			})
		}
		return true
	})

	return utils.MakeStrSet(r...)
}

func walkPassthrough(excludes, passthroughs utils.StrSet, dep *AppDependency, integrations *DependencySet, module *sysl.Module) {
	target := dep.Target
	targetName := target.Name
	targetEndpoint := target.Endpoint
	excludedApps := utils.MakeStrSet(targetName).Intersection(excludes)
	undeterminedEps := utils.MakeStrSet(targetEndpoint).Intersection(utils.MakeStrSet(".. * <- *", "*"))
	//Add to integration array since all dependencies are determined.
	if len(excludedApps) == 0 && len(undeterminedEps) == 0 {
		integrations.Deps[dep] = struct{}{}
	}

	// find the next outbound dep
	if passthroughs.Contains(targetName) {
		cs := NewCallSlice()
		cs.CollectCallStatements(module.GetApps()[targetName].GetEndpoints()[targetEndpoint].GetStmt())
		for _, call := range cs.GetSlice() {
			nextAppName := strings.Join(call.GetTarget().GetPart(), " :: ")
			nextEpName := call.GetEndpoint()
			next := MakeAppElement(nextAppName, nextEpName)
			nextDep := MakeAppDependency(target, next)
			walkPassthrough(excludes, passthroughs, nextDep, integrations, module)
		}
	}
}

func (ds *DependencySet) buildArray() []*AppDependency {
	r := []*AppDependency{}
	for k := range ds.Deps {
		r = append(r, k)
	}

	return r
}

func (ds *DependencySet) FindIntegrations(apps, excludes, passthroughs utils.StrSet, module *sysl.Module) *DependencySet {
	integrations := NewDependencySet()
	outboundDeps := NewDependencySet()
	lenPassthroughs := len(passthroughs)
	for dep := range ds.Deps {
		appNames := dep.extractAppNames()
		endpoints := dep.extractEndpoints()
		isSubsection := isSub(appNames, apps)
		isSelfSubsection := isSub(utils.MakeStrSet(dep.Self.Name), apps)
		isTargetSubsection := isSub(utils.MakeStrSet(dep.Target.Name), passthroughs)
		interExcludes := appNames.Intersection(excludes)
		interEndpoints := endpoints.Intersection(utils.MakeStrSet(".. * <- *", "*"))
		lenInterExcludes := len(interExcludes)
		lenInterEndpoints := len(interEndpoints)
		if isSubsection && lenInterExcludes == 0 && lenInterEndpoints == 0 {
			integrations.Deps[dep] = struct{}{}
		}
		// collect outbound dependencies
		if lenPassthroughs > 0 &&
			((isSubsection || (isSelfSubsection && isTargetSubsection)) && lenInterExcludes == 0 && lenInterEndpoints == 0) {
			outboundDeps.Deps[dep] = struct{}{}
		}
	}
	if lenPassthroughs > 0 {
		for dep := range outboundDeps.Deps {
			walkPassthrough(excludes, passthroughs, dep, integrations, module)
		}
	}

	return integrations
}

func (ds *DependencySet) ResolveDependencies(module *sysl.Module) {
	apps := module.GetApps()
	for appname, app := range apps {
		for epname, endpoint := range app.GetEndpoints() {
			ds.resolveStatementDependencies(endpoint.GetStmt(), apps, appname, epname)
		}
	}

	if len(ds.errors) > 0 {
		panic(fmt.Sprintf("broken deps:\n  %s", strings.Join(ds.errors, "\n")))
	}
}

func (ds *DependencySet) resolveStatementDependencies(stmts []*sysl.Statement, apps map[string]*sysl.Application, appname, epname string) {
	for _, stat := range stmts {
		switch c := stat.GetStmt().(type) {
		case *sysl.Statement_Call:
			var errStr string
			targetName := getAppName(c.Call.GetTarget())
			targetApp := apps[targetName]
			if targetApp == nil {
				errStr = fmt.Sprintf("%s <- %s: calls non-existent app %s", appname, epname, targetName)
				ds.errors = append(ds.errors, errStr)
			} else {
				isValid := !hasPattern("abstract", utils.MakeStrSetFromSpecificAttr("patterns", targetApp.GetAttrs()).ToSlice())
				if !isValid {
					panic(fmt.Sprintf("call target '%s' must not be ~abstract", targetName))
				}
				callEndpoint := c.Call.GetEndpoint()
				if targetApp.GetEndpoints()[callEndpoint] == nil {
					errStr = fmt.Sprintf(
						"%s <- %s: calls non-existent endpoint %s -> %s", appname, epname, targetName, callEndpoint)
					ds.errors = append(ds.errors, errStr)
				} else {
					selfApp := MakeAppElement(appname, epname)
					targetApp := MakeAppElement(targetName, callEndpoint)
					dep := MakeAppDependency(selfApp, targetApp)
					ds.Deps[dep] = struct{}{}
				}
			}
		case *sysl.Statement_Action, *sysl.Statement_Ret:
			continue
		case *sysl.Statement_Cond:
			ds.resolveStatementDependencies(c.Cond.GetStmt(), apps, appname, epname)
		case *sysl.Statement_Loop:
			ds.resolveStatementDependencies(c.Loop.GetStmt(), apps, appname, epname)
		case *sysl.Statement_LoopN:
			ds.resolveStatementDependencies(c.LoopN.GetStmt(), apps, appname, epname)
		case *sysl.Statement_Foreach:
			ds.resolveStatementDependencies(c.Foreach.GetStmt(), apps, appname, epname)
		case *sysl.Statement_Group:
			ds.resolveStatementDependencies(c.Group.GetStmt(), apps, appname, epname)
		case *sysl.Statement_Alt:
			for _, choice := range c.Alt.GetChoice() {
				ds.resolveStatementDependencies(choice.GetStmt(), apps, appname, epname)
			}
		default:
			panic("No statement!")
		}
	}
}

func isSub(child, parent utils.StrSet) bool {
	if len(child) == 0 && len(parent) == 0 {
		return true
	}
	if len(parent) == 0 {
		return false
	}
	if len(child) == 0 {
		return true
	}
	return len(child.Difference(parent)) == 0
}

func hasPattern(s string, arr []string) bool {
	m := buildStringBoolFilter(arr)

	return m[s]
}

func toPattern(comp []string) string {
	return fmt.Sprintf(`^(?:%s)(?: *::|$)`, strings.Join(comp, "|"))
}

func getAppName(appname *sysl.AppName) string {
	return strings.Join(appname.Part, " :: ")
}

func buildStringBoolFilter(a []string) map[string]bool {
	m := make(map[string]bool)
	for _, v := range a {
		m[v] = true
	}

	return m
}

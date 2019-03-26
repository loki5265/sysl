package main

import (
	"flag"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/anz-bank/sysl/src/proto"
	"github.com/ghemawat/stream"
	log "github.com/sirupsen/logrus"
)

type AppElement struct {
	Name     string
	Endpoint string
}

type AppDependency struct {
	Self   *AppElement
	Target *AppElement
}

func MakeAppDependency(self, target *AppElement) *AppDependency {
	return &AppDependency{self, target}
}

func MakeAppElement(name, endpoint string) *AppElement {
	return &AppElement{name, endpoint}
}

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

func makeCalls(statements []*sysl.Statement, calls []*sysl.Call) {
	for _, stat := range statements {
		switch c := stat.GetStmt().(type) {
		case *sysl.Statement_Call:
			calls = append(calls, c.Call)
		case *sysl.Statement_Action:
			continue
		case *sysl.Statement_Cond:
			makeCalls(c.Cond.GetStmt(), calls)
		case *sysl.Statement_Loop:
			makeCalls(c.Loop.GetStmt(), calls)
		case *sysl.Statement_LoopN:
			makeCalls(c.LoopN.GetStmt(), calls)
		case *sysl.Statement_Foreach:
			makeCalls(c.Foreach.GetStmt(), calls)
		case *sysl.Statement_Group:
			makeCalls(c.Group.GetStmt(), calls)
		case *sysl.Statement_Alt:
			for _, choice := range c.Alt.GetChoice() {
				makeCalls(choice.GetStmt(), calls)
			}
		case *sysl.Statement_Ret:
			continue
		default:
			panic("No statement!")
		}
	}
}

func formatAppName(appNames []string) string {
	return strings.Join(appNames, " :: ")
}

func validatePatterns(matched string, patterns []string) bool {
	isValid := true
	for _, pattern := range patterns {
		if pattern == matched {
			isValid = false
			break
		}
	}

	return isValid
}

func checkDependencies(module *sysl.Module) []*AppDependency {
	var errStr string
	deps := []*AppDependency{}
	errors := []string{}
	apps := module.GetApps()
	for appname, app := range module.GetApps() {
		for epname, endpoint := range app.GetEndpoints() {
			calls := []*sysl.Call{}
			makeCalls(endpoint.GetStmt(), calls)
			for _, call := range calls {
				targetName := formatAppName(call.GetTarget().GetPart())
				targetApp := apps[targetName]
				if targetApp == nil {
					errStr = fmt.Sprintf("%s <- %s: calls non-existent app %s", appname, epname, targetName)
					errors = append(errors, errStr)
				} else {
					isValid := validatePatterns("abstract", extractApplicationAttr("patterns", targetApp))
					if !isValid {
						panic(fmt.Sprintf("call target '%s' must not be ~abstract", targetName))
					}
					callEndpoint := call.GetEndpoint()
					if targetApp.GetEndpoints()[callEndpoint] == nil {
						errStr = fmt.Sprintf(
							"%s <- %s: calls non-existent endpoint %s -> %s", appname, epname, targetName, callEndpoint)
					} else {
						selfApp := MakeAppElement(appname, epname)
						targetApp := MakeAppElement(targetName, callEndpoint)
						dep := MakeAppDependency(selfApp, targetApp)
						deps = append(deps, dep)
					}
				}
			}
		}
	}

	if len(errors) > 0 {
		panic(fmt.Sprintf("broken deps:\n  %s", strings.Join(errors, "\n")))
	}
	return deps
}

func extractAppNames(dep *AppDependency) []string {
	apps := []string{}
	apps = append(apps, dep.Self.Name)
	apps = append(apps, dep.Target.Name)

	return apps
}

func extractEndpoints(dep *AppDependency) []string {
	eps := []string{}
	eps = append(eps, dep.Self.Endpoint)
	eps = append(eps, dep.Target.Endpoint)

	return eps
}

func arrayToMap(a []string) map[string]bool {
	m := make(map[string]bool)
	for _, v := range a {
		m[v] = true
	}

	return m
}

func intersection(a, b []string) []string {
	result := []string{}
	m := arrayToMap(a)
	for _, k := range b {
		if _, ok := m[k]; ok {
			result = append(result, k)
		}
	}

	return result
}

// execute like a - b
func subtraction(a, b []string) []string {
	result := []string{}
	m := arrayToMap(b)
	for _, k := range a {
		if _, has := m[k]; !has {
			result = append(result, k)
		}
	}

	return result
}

func isSub(child, parent []string) bool {
	if parent == nil || len(parent) == 0 {
		return false
	}
	if child == nil || len(child) == 0 {
		return true
	}
	return len(subtraction(parent, child)) == 0
}

func contains(a string, arr []string) bool {
	m := arrayToMap(arr)

	return m[a]
}

func toPattern(comp []string) string {
	return fmt.Sprintf(`^(?:%s)(?: *::|$)`, strings.Join(comp, "|"))
}

func findMatchingApps(module *sysl.Module, excludes []string, integrations []string, deps []*AppDependency) []string {
	result := []string{}
	appReStr := toPattern(integrations)
	re := regexp.MustCompile(appReStr)
	for _, apps := range deps {
		appNames := extractAppNames(apps)
		inter := intersection(appNames, excludes)
		if len(inter) > 0 {
			continue
		}
		filtered, err := stream.Contents(stream.Items(appNames...), stream.If(func(item string) bool {
			app := module.GetApps()[item]
			return re.MatchString(item) && validatePatterns("human", extractApplicationAttr("patterns", app))
		}))
		if err != nil {
			log.Error(err)
		}
		result = append(result, filtered...)
	}

	return result
}

func findApps(module *sysl.Module, excludes, matchingApps []string, deps []*AppDependency) []string {
	result := []string{}
	for _, apps := range deps {
		appNames := extractAppNames(apps)
		interExcludes := intersection(appNames, excludes)
		if len(interExcludes) > 0 {
			continue
		}
		interMatchings := intersection(appNames, matchingApps)
		if len(interMatchings) == 0 {
			continue
		}
		filtered, err := stream.Contents(stream.Items(appNames...), stream.If(func(item string) bool {
			app := module.GetApps()[item]
			return validatePatterns("human", extractApplicationAttr("patterns", app))
		}))
		if err != nil {
			log.Error(err)
		}
		result = append(result, filtered...)
	}

	return result
}

func walkPassthrough(excludes, passthroughs []string, dep *AppDependency, integrations []*AppDependency, module *sysl.Module) {
	target := dep.Target
	targetName := target.Name
	targetEndpoint := target.Endpoint
	interApp := intersection([]string{targetName}, excludes)
	interEndpoint := intersection([]string{targetEndpoint}, excludes)
	// add to integration array
	if len(interApp) == 0 && len(interEndpoint) == 0 {
		integrations = append(integrations, dep)
	}

	// find the next outbound dep
	if contains(targetName, passthroughs) {
		calls := []*sysl.Call{}
		endpointStmts := module.GetApps()[targetName].GetEndpoints()[targetEndpoint].GetStmt()
		makeCalls(endpointStmts, calls)
		for _, call := range calls {
			nextAppName := strings.Join(call.GetTarget().GetPart(), " :: ")
			nextEpName := call.GetEndpoint()
			next := MakeAppElement(nextAppName, nextEpName)
			nextDep := MakeAppDependency(target, next)
			walkPassthrough(excludes, passthroughs, nextDep, integrations, module)
		}
	}
}

func findIntegrations(apps, excludes, passthroughs []string, deps []*AppDependency, module *sysl.Module) []*AppDependency {
	integrations := []*AppDependency{}
	outboundDeps := []*AppDependency{}
	lenPassthroughs := len(passthroughs)
	for _, dep := range deps {
		appNames := extractAppNames(dep)
		endpoints := extractEndpoints(dep)
		isSubsection := isSub(appNames, apps)
		isSelfSubsection := isSub([]string{appNames[0]}, apps)
		isTargetSubsection := isSub([]string{appNames[1]}, apps)
		interExcludes := intersection(appNames, excludes)
		interEndpoints := intersection(endpoints, []string{".. * <- *", "*"})
		lenInterExcludes := len(interExcludes)
		lenInterEndpoints := len(interEndpoints)
		if isSubsection && lenInterExcludes == 0 && lenInterEndpoints == 0 {
			integrations = append(integrations, dep)
		}
		// collect outbound dependencies
		if lenPassthroughs > 0 &&
			(isSubsection || isSelfSubsection && isTargetSubsection && lenInterExcludes == 0 && lenInterEndpoints == 0) {
			outboundDeps = append(outboundDeps, dep)
		}
	}
	if lenPassthroughs > 0 {
		for _, dep := range outboundDeps {
			walkPassthrough(excludes, passthroughs, dep, integrations, module)
		}
	}

	return integrations
}

func loadApp(root string, models []string) *sysl.Module {
	// Model we want to generate ints for, the first non-empty model
	var model string
	for _, val := range models {
		if len(val) > 0 {
			model = val
			break
		}
	}
	mod, err := Parse(model, root)
	if err == nil {
		return mod
	}
	log.Errorf("unable to load module:\n\troot: " + root + "\n\tmodel:" + model)
	return nil
}

func extractApplicationAttr(attrStr string, app *sysl.Application) []string {
	return transformAttributes(app.GetAttrs()[attrStr])
}

func extractEndpointAttr(attrStr string, endpt *sysl.Endpoint) []string {
	return transformAttributes(endpt.GetAttrs()[attrStr])
}

func transformAttributes(attribute *sysl.Attribute) []string {
	result := []string{}
	attrs := []*sysl.Attribute{}
	if attribute != nil {
		attrs = attribute.GetA().GetElt()
	}
	for _, v := range attrs {
		result = append(result, v.GetS())
	}

	return result
}

func extractAction(stmts []*sysl.Statement) []string {
	result := []string{}
	for _, v := range stmts {
		action := v.GetAction()
		if action == nil {
			panic("No action statement!")
		}
		result = append(result, action.GetAction())
	}

	return result
}

func GenerateIntegrations(
	root_model, title, plantuml, output, project, filter string,
	exclude, modules []string,
	clustered, epa bool,
) {
	mod := loadApp(root_model, modules)
	if len(exclude) == 0 && project != "" {
		exclude = append(exclude, project)
	}
	deps := checkDependencies(mod)

	var out_fmt func(output string) string
	// The "project" app that specifies the required view of the integrations
	app := mod.GetApps()[project]
	// Interate over each endpoint within the selected project
	for _, endpt := range app.GetEndpoints() {
		// build the set of excluded items
		excludes := extractEndpointAttr("exclude", endpt)
		passthroughs := extractEndpointAttr("passthrough", endpt)
		// endpt.stmt's "action" will conatain the "apps" whose integration is to be drawn
		// each one of these will be placed into the "integrations" list
		integrations := extractAction(endpt.GetStmt())

		matchingApps := findMatchingApps(mod, exclude, integrations, deps)
		apps := findApps(mod, exclude, matchingApps, deps)
		apps = subtraction(apps, excludes)
		apps = subtraction(apps, passthroughs)
		output = out_fmt(output)

		if filter != "" {
			re := regexp.MustCompile(filter)
			if !re.MatchString(output) {
				continue
			}
		}

		// invoke generate_view string
	}
}

func DoGenerateIntegrations(stdout, stderr io.Writer, flags *flag.FlagSet, args []string) {
	defer func() {
		if err := recover(); err != nil {
			log.Errorln(err)
		}
	}()
	var exclude, modules_flag arrayFlags
	root_model := flags.String("root-model", ".", "sysl root directory for input model file (default: .)")
	title := flags.String("title", "", "diagram title")
	plantuml := flags.String("plantuml", "", strings.Join([]string{"base url of plantuml server",
		"(default: $SYSL_PLANTUML or http://localhost:8080/plantuml",
		"see http://plantuml.com/server.html#install for more info)"}, "\n"))
	output := flags.String("output", "%(epname).png", "output file(default: %(epname).png)")
	project := flags.String("project", "", "project pseudo-app to render")
	filter := flags.String("filter", "", "Only generate diagrams whose output paths match a pattern")
	flags.Var(&exclude, "exclude", "apps to exclude")
	flags.Var(&modules_flag, "modules", strings.Join([]string{"input files without .sysl extension and with leading /",
		"eg: /project_dir/my_models",
		"combine with --root if needed"}, "\n"))
	clustered := flags.Bool("clustered", false, "group integration components into clusters")
	epa := flags.Bool("epa", false, "produce and EPA integration view")

	// Following variables currently are unused. Keep them to align with the python version.
	expire_cache := flags.Bool("expire-cache", false, "Expire cache entries to force checking against real destination(default: false)")
	dry_run := flags.Bool("dry-run", false, "Don't perform confluence uploads, but show what would have happened(default: false)")
	verbose := flags.Bool("verbose", false, "Report each output(default: false)")

	err := flags.Parse(args[1:])
	if err != nil {
		log.Errorf("arguments parse error: %v", err)
	}
	log.Debugf("root_model: %s\n", *root_model)
	log.Debugf("project: %v\n", project)
	log.Debugf("clustered: %t\n", *clustered)
	log.Debugf("epa: %t\n", *epa)
	log.Debugf("title: %s\n", *title)
	log.Debugf("plantuml: %s\n", *plantuml)
	log.Debugf("verbose: %t\n", *verbose)
	log.Debugf("expire_cache: %t\n", *expire_cache)
	log.Debugf("dry_run: %t\n", *dry_run)
	log.Debugf("filter: %s\n", *filter)
	log.Debugf("modules: %s\n", modules_flag)
	log.Debugf("output: %s\n", *output)

	GenerateIntegrations(*root_model, *title, *plantuml, *output, *project, *filter, exclude, modules_flag, *clustered, *epa)
}

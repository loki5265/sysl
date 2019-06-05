package main

import (
	"flag"
	"io"
	"regexp"
	"strings"

	"github.com/anz-bank/sysl/sysl2/sysl/integration"
	"github.com/anz-bank/sysl/sysl2/sysl/seqs"
	"github.com/anz-bank/sysl/sysl2/sysl/utils"
	log "github.com/sirupsen/logrus"
)

func GenerateIntegrations(
	root_model, title, output, project, filter, modules string,
	exclude []string, clustered, epa bool,
) map[string]string {
	r := make(map[string]string)
	mod := loadApp(root_model, modules)

	if len(exclude) == 0 && project != "" {
		exclude = append(exclude, project)
	}
	excludeStrSet := utils.MakeStrSet(exclude...)
	ds := integration.NewDependencySet()
	ds.ResolveDependencies(mod)

	//var out_fmt func(output string) string
	// The "project" app that specifies the required view of the integration
	app := mod.GetApps()[project]
	of := seqs.MakeFormatParser(output)
	// Interate over each endpoint within the selected project
	for epname, endpt := range app.GetEndpoints() {
		// build the set of excluded items
		excludes := utils.MakeStrSetFromSpecificAttr("exclude", endpt.GetAttrs())
		passthroughs := utils.MakeStrSetFromSpecificAttr("passthrough", endpt.GetAttrs())
		// endpt.stmt's "action" will conatain the "apps" whose integration is to be drawn
		// each one of these will be placed into the "integration" list
		integrations := utils.MakeStrSetFromActionStatement(endpt.GetStmt())

		highlights := integration.FindApps(mod, excludeStrSet, integrations, ds, true)
		apps := integration.FindApps(mod, excludeStrSet, highlights, ds, false)
		apps = apps.Difference(excludes)
		apps = apps.Difference(passthroughs)
		output_dir := of.FmtOutput(project, epname, endpt.GetLongName(), endpt.GetAttrs())

		if filter != "" {
			re := regexp.MustCompile(filter)
			if !re.MatchString(output) {
				continue
			}
		}

		// invoke generate_view string
		dependencySet := ds.FindIntegrations(apps, excludes, passthroughs, mod)
		deps := []*integration.AppDependency{}
		for dep := range dependencySet.Deps {
			deps = append(deps, dep)
		}
		intsParam := integration.MakeIntsParam(apps.ToSlice(), highlights, deps, app, endpt)
		args := integration.MakeArgs(title, project, clustered, epa)
		r[output_dir] = integration.GenerateView(args, intsParam, mod)
	}

	return r
}

func DoGenerateIntegrations(stdout, stderr io.Writer, flags *flag.FlagSet, args []string) {
	defer func() {
		if err := recover(); err != nil {
			log.Errorln(err)
		}
	}()
	var exclude arrayFlags
	root_model := flags.String("root-model", ".", "sysl root directory for input model file (default: .)")
	title := flags.String("title", "", "diagram title")
	plantuml := flags.String("plantuml", "", strings.Join([]string{"base url of plantuml server",
		"(default: $SYSL_PLANTUML or http://localhost:8080/plantuml",
		"see http://plantuml.com/server.html#install for more info)"}, "\n"))
	output := flags.String("output", "%(epname).png", "output file(default: %(epname).png)")
	project := flags.String("project", "", "project pseudo-app to render")
	filter := flags.String("filter", "", "Only generate diagrams whose output paths match a pattern")
	modules := flags.String("modules", ".", strings.Join([]string{"input files without .sysl extension and with leading /",
		"eg: /project_dir/my_models",
		"combine with --root if needed"}, "\n"))
	flags.Var(&exclude, "exclude", "apps to exclude")
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
	log.Debugf("modules: %s\n", *modules)
	log.Debugf("output: %s\n", *output)

	r := GenerateIntegrations(*root_model, *title, *output, *project, *filter, *modules, exclude, *clustered, *epa)
	for k, v := range r {
		OutputPlantuml(k, *plantuml, v)
	}
}

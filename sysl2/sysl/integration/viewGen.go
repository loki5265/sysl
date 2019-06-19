package integration

import (
	"fmt"
	"github.com/anz-bank/sysl/src/proto"
	"strconv"
	"strings"
	"sysl/sysl2/sysl/utils"
)

type IntsParam struct {
	apps         []string
	highlights   map[string]struct{}
	integrations []*AppDependency
	app          *sysl.Application
	endpt        *sysl.Endpoint
}

type Args struct {
	title     string
	project   string
	clustered bool
	epa       bool
}

type IntsDiagramVisitor struct {
	m          *sysl.Module
	sb         *strings.Builder
	highlights map[string]struct{}
	symbols    map[string]*_var
	topSymbols map[string]*_topVar
	project    string
}

type _var struct {
	label string
	alias string
}

type _topVar struct {
	topLabel string
	topAlias string
}

type Statements struct {
	stmtSlice []*sysl.Statement
}

type CallsDrawn struct {
	Self   string
	Target string
}

type viewParams struct {
	restrictBy         string
	endptAttrs         map[string]*sysl.Attribute
	highLightColor     string
	arrowColor         string
	indirectArrowColor string
	diagramTitle       string
}

func NewStatements() *Statements {
	return &Statements{[]*sysl.Statement{}}
}

func MakeIntsParam(apps []string, highlights map[string]struct{}, dependencies []*AppDependency, app *sysl.Application, endpt *sysl.Endpoint) *IntsParam {
	return &IntsParam{apps, highlights, dependencies, app, endpt}
}

func MakeArgs(title, project string, clustered, epa bool) *Args {
	return &Args{title, project, clustered, epa}
}

func MakeIntsDiagramVisitor(m *sysl.Module, sb *strings.Builder, highlights map[string]struct{}, project string) *IntsDiagramVisitor {
	return &IntsDiagramVisitor{
		m:          m,
		sb:         sb,
		highlights: highlights,
		symbols:    map[string]*_var{},
		topSymbols: map[string]*_topVar{},
		project:    project,
	}
}

func (v *IntsDiagramVisitor) VarManagerForComponent(appName string, nameMap map[string]string) string {
	if key, ok := nameMap[appName]; ok {
		appName = key
	}
	if s, ok := v.symbols[appName]; ok {
		return s.alias
	}

	i := len(v.symbols)
	alias := fmt.Sprintf("_%d", i)
	attrs := getApplicationAttrs(v.m, appName)
	attrs["appname"] = appName
	label := utils.ParseFmt(attrs, v.m.Apps[v.project].GetAttrs()["appfmt"].GetS())
	s := &_var{
		label: label,
		alias: alias,
	}
	v.symbols[appName] = s
	r := fmt.Sprintf("[%s] as %s", label, alias)
	if _, ok := v.highlights[appName]; ok {
		r += " <<highlight>>"
	}
	fmt.Fprintln(v.sb, r)
	return s.alias
}

func (v *IntsDiagramVisitor) VarManagerForTopState(appName string) string {
	var alias, label string
	attrs := map[string]string{}
	if ts, ok := v.topSymbols[appName]; ok {
		return ts.topAlias
	}
	i := len(v.topSymbols)
	alias = fmt.Sprintf("_%d", i)

	attrs = getApplicationAttrs(v.m, appName)
	attrs["appname"] = appName
	label = utils.ParseFmt(attrs, v.m.Apps[v.project].GetAttrs()["appfmt"].GetS())
	ts := &_topVar{
		topLabel: label,
		topAlias: alias,
	}
	v.topSymbols[appName] = ts
	r := ""
	if _, ok := v.highlights[appName]; ok {
		r = fmt.Sprintf("state \"%s\" as X%s <<highlight>> {", label, alias)
	} else {
		r = fmt.Sprintf("state \"%s\" as X%s {", label, alias)
	}
	fmt.Fprintln(v.sb, r)

	return ts.topAlias
}

func (v *IntsDiagramVisitor) VarManagerForState(name string) string {
	var appName, alias, label string
	attrs := map[string]string{}

	appName = strings.Split(name, " : ")[0]
	epName := strings.Split(name, " : ")[1]

	if s, ok := v.symbols[name]; ok {
		return s.alias
	}
	i := len(v.symbols)
	alias = fmt.Sprintf("_%d", i)

	if v.m.Apps[appName].Endpoints[epName] != nil {
		for k, v := range v.m.Apps[appName].Endpoints[epName].Attrs {
			attrs["@"+k] = v.GetS()
		}
	}
	attrs["appname"] = epName
	label = utils.ParseFmt(attrs, v.m.Apps[v.project].GetAttrs()["appfmt"].GetS())
	s := &_var{
		label: label,
		alias: alias,
	}
	v.symbols[name] = s
	r := fmt.Sprintf("  state \"%s\" as %s", label, alias)
	if _, ok := v.highlights[name]; ok {
		r += " <<highlight>>"
	}
	fmt.Fprintln(v.sb, r)
	return s.alias
}

func (v *IntsDiagramVisitor) buildClusterForStateView(deps []*AppDependency, restrictBy string) {
	clusters := map[string][]string{}
	for _, dep := range deps {
		appA := dep.Self.Name
		appB := dep.Target.Name
		epA := dep.Self.Endpoint
		epB := dep.Target.Endpoint
		_, okA := v.m.Apps[appA].Endpoints[epA].Attrs[restrictBy]
		_, okB := v.m.Apps[appB].Endpoints[epB].Attrs[restrictBy]
		if _, ok := v.m.Apps[appA].Attrs[restrictBy]; !ok && restrictBy != "" {
			if _, ok := v.m.Apps[appB].Attrs[restrictBy]; !ok {
				continue
			}
		}
		if !okA && restrictBy != "" && !okB {
			continue
		}
		clusters[appA] = append(clusters[appA], epA)
		if appA != appB && !v.m.Apps[appA].Endpoints[epA].IsPubsub {
			clusters[appA] = append(clusters[appA], epB+" client")
		}
		clusters[appB] = append(clusters[appB], epB)
	}

	for k, apps := range clusters {
		v.VarManagerForTopState(k)
		strSet := utils.MakeStrSet(apps...)
		for _, m := range strSet.ToSlice() {
			v.VarManagerForState(k + " : " + m)
		}
		fmt.Fprintln(v.sb, "}")
	}
}

func (v *IntsDiagramVisitor) buildClusterForComponentView(apps []string) map[string]string {
	nameMap := map[string]string{}
	clusters := map[string][]string{}
	for _, v := range apps {
		cluster := strings.Split(v, " :: ")
		if len(cluster) > 1 {
			clusters[cluster[0]] = append(clusters[cluster[0]], v)
		}
	}

	for k, v := range clusters {
		if len(v) <= 1 {
			delete(clusters, k)
		}
		for _, s := range v {
			nameMap[s] = strings.Split(s, " :: ")[1]
		}
	}

	for k, apps := range clusters {
		fmt.Fprintf(v.sb, "package \"%s\" {", k)
		fmt.Fprintln(v.sb)
		for _, n := range apps {
			v.VarManagerForComponent(n, nameMap)
		}
		fmt.Fprintln(v.sb, "}")
	}

	return nameMap
}

func (v *IntsDiagramVisitor) generateStateView(args *Args, viewParams viewParams, params *IntsParam) string {

	fmt.Fprintln(v.sb, "@startuml")
	if viewParams.diagramTitle != "" {
		fmt.Fprintln(v.sb, "title "+viewParams.diagramTitle)
	}
	fmt.Fprintln(v.sb, "left to right direction")
	fmt.Fprintln(v.sb, "scale max 16384 height")
	fmt.Fprintln(v.sb, "hide empty description")
	fmt.Fprintln(v.sb, "skinparam state {")
	fmt.Fprintln(v.sb, "  BackgroundColor FloralWhite")
	fmt.Fprintln(v.sb, "  BorderColor Black")
	fmt.Fprintln(v.sb, "  ArrowColor Crimson")
	if viewParams.highLightColor != "" {
		fmt.Fprintln(v.sb, "  BackgroundColor<<highlight>> "+viewParams.highLightColor)
	}
	if viewParams.arrowColor != "" {
		fmt.Fprintln(v.sb, "  ArrowColor "+viewParams.arrowColor)
	}

	if viewParams.indirectArrowColor != "" && viewParams.indirectArrowColor != "none" {
		fmt.Fprintln(v.sb, "  ArrowColor<<indirect>> "+viewParams.indirectArrowColor)
		fmt.Fprintln(v.sb, "  ArrowColor<<internal>> "+viewParams.indirectArrowColor)
	}
	fmt.Fprintln(v.sb, "}")

	v.buildClusterForStateView(params.integrations, viewParams.restrictBy)
	var processed []string
	for _, dep := range params.integrations {
		appA := dep.Self.Name
		appB := dep.Target.Name
		epA := dep.Self.Endpoint
		epB := dep.Target.Endpoint
		_, restrictByAppA := v.m.Apps[appA].Attrs[viewParams.restrictBy]
		_, restrictByAppB := v.m.Apps[appB].Attrs[viewParams.restrictBy]
		_, restrictByEpA := v.m.Apps[appA].Endpoints[epA].Attrs[viewParams.restrictBy]
		_, restrictByEpB := v.m.Apps[appB].Endpoints[epB].Attrs[viewParams.restrictBy]
		if viewParams.restrictBy != "" && !restrictByAppA && !restrictByAppB {
			continue
		}
		if viewParams.restrictBy != "" && !restrictByEpA && !restrictByEpB {
			continue
		}
		matchApp := appB
		matchEp := epB
		label := ""
		needsInt := appA != matchApp

		var pubSubSrcPtrns []string
		if v.m.Apps[appA].Endpoints[epA].Attrs["patterns"] != nil {
			for _, v := range v.m.Apps[appA].Endpoints[epA].Attrs["patterns"].GetA().Elt {
				pubSubSrcPtrns = append(pubSubSrcPtrns, v.GetS())
			}
		}

		tgtPtrns := utils.MakeStrSet()
		if v.m.Apps[matchApp].Endpoints[matchEp].Attrs["patterns"] != nil {
			for _, v := range v.m.Apps[matchApp].Endpoints[matchEp].Attrs["patterns"].GetA().Elt {
				tgtPtrns.Insert(v.GetS())
			}
		} else {
			if v.m.Apps[matchApp].Attrs["patterns"] != nil {
				for _, v := range v.m.Apps[matchApp].Attrs["patterns"].GetA().Elt {
					tgtPtrns.Insert(v.GetS())
				}
			}
		}
		stmts := NewStatements()
		stmts.makeStmts(v.m.Apps[appA].Endpoints[epA].Stmt)
		for _, stmt := range stmts.stmtSlice {
			appBName := strings.Join(stmt.GetCall().GetTarget().GetPart(), " :: ")
			if matchApp == appBName && matchEp == stmt.GetCall().Endpoint {
				fmtVars := stmt.GetAttrs()
				attrs := map[string]string{}

				for k, v := range stmt.GetAttrs() {
					attrs["@"+k] = v.GetS()
				}
				var srcPtrns []string
				if fmtVars["patterns"] != nil {
					for _, v := range fmtVars["patterns"].GetA().Elt {
						srcPtrns = append(srcPtrns, v.GetS())
					}
				} else {
					srcPtrns = pubSubSrcPtrns
				}
				var ptrns string
				if srcPtrns != nil || tgtPtrns != nil {
					ptrns = strings.Join(srcPtrns, ", ") + " → " + strings.Join(tgtPtrns.ToSlice(), ", ")
				} else {
					ptrns = ""
				}
				attrs["patterns"] = ptrns
				if needsInt {
					attrs["needs_int"] = strconv.FormatBool(needsInt)
				}
				label = utils.ParseFmt(attrs, params.app.Attrs["epfmt"].GetS())
			}
		}

		flow := strings.Join([]string{appA, epB, appB, epB}, ".")
		isPubSub := v.m.Apps[appA].Endpoints[epA].GetIsPubsub()
		epBClient := epB + " client"

		if appA != appB {
			if isPubSub {
				if label != "" {
					label = " : " + label
				}
				fmt.Fprintf(v.sb, "%s -%s> %s%s", v.VarManagerForState(appA+" : "+epA), "[#blue]", v.VarManagerForState(appB+" : "+epB), label)
				fmt.Fprintln(v.sb)
			} else {
				color := ""
				if viewParams.indirectArrowColor == "" {
					color = "[#silver]-"
				} else {
					color = "[#" + viewParams.indirectArrowColor + "]-"
				}
				fmt.Fprintf(v.sb, "%s -%s> %s", v.VarManagerForState(appA+" : "+epA), color, v.VarManagerForState(appA+" : "+epBClient))
				fmt.Fprintln(v.sb)
				if !stringInSlice(flow, processed) {
					fmt.Fprintf(v.sb, "%s -%s> %s : %s", v.VarManagerForState(appA+" : "+epBClient), "[#black]", v.VarManagerForState(appB+" : "+epB), label)
					fmt.Fprintln(v.sb)
					processed = append(processed, flow)
				}
			}
		} else {
			color := ""
			if viewParams.indirectArrowColor == "" {
				color = "[#silver]-"
			} else {
				color = "[#" + viewParams.indirectArrowColor + "]-"
			}
			fmt.Fprintf(v.sb, "%s -%s> %s%s", v.VarManagerForState(appA+" : "+epA), color, v.VarManagerForState(appB+" : "+epB), label)
			fmt.Fprintln(v.sb)
		}
	}
	fmt.Fprintln(v.sb, "@enduml")
	return v.sb.String()

}

func (v *IntsDiagramVisitor) drawComponentView(viewParams viewParams, params *IntsParam, nameMap map[string]string) {
	callsDrawn := map[CallsDrawn]struct{}{}
	if viewParams.endptAttrs["view"].GetS() == "system" {
		for _, dep := range params.integrations {
			appA := dep.Self.Name
			appB := dep.Target.Name
			apps := &CallsDrawn{
				Self:   appA,
				Target: appB,
			}
			var direct []string
			if _, ok := params.highlights[appA]; ok {
				direct = append(direct, appA)
			}
			if _, ok := params.highlights[appB]; ok {
				direct = append(direct, appB)
			}
			appA = strings.Split(appA, " :: ")[0]
			appB = strings.Split(appB, " :: ")[0]
			_, ok := callsDrawn[*apps]
			if appA != appB && !ok {
				if direct != nil || viewParams.indirectArrowColor != "none" {
					indirect := ""
					if direct == nil {
						indirect = " <<indirect>>"
					}
					fmt.Fprintf(v.sb, "%s --> %s%s", v.VarManagerForComponent(appA, nameMap), v.VarManagerForComponent(appB, nameMap), indirect)
					fmt.Fprintln(v.sb)
					callsDrawn[*apps] = struct{}{}
				}
			}
		}

	} else {
		for _, dep := range params.integrations {
			appA := dep.Self.Name
			appB := dep.Target.Name
			apps := &CallsDrawn{
				Self:   appA,
				Target: appB,
			}
			var direct []string
			if _, ok := params.highlights[appA]; ok {
				direct = append(direct, appA)
			}
			if _, ok := params.highlights[appB]; ok {
				direct = append(direct, appB)
			}
			_, ok := callsDrawn[*apps]
			if appA != appB && !ok {
				if direct != nil || viewParams.indirectArrowColor != "none" {
					indirect := ""
					if direct == nil {
						indirect = " <<indirect>>"
					}
					fmt.Fprintf(v.sb, "%s --> %s%s", v.VarManagerForComponent(appA, nameMap), v.VarManagerForComponent(appB, nameMap), indirect)
					fmt.Fprintln(v.sb)
					callsDrawn[*apps] = struct{}{}
				}
			}
		}
		for _, app := range params.apps {
			for _, mixin := range v.m.Apps[app].GetMixin2() {
				mixinName := strings.Join(mixin.Name.Part, " :: ")
				fmt.Fprintf(v.sb, "%s <|.. %s", v.VarManagerForComponent(mixinName, nameMap), v.VarManagerForComponent(app, nameMap))
				fmt.Fprintln(v.sb)
			}
		}
	}
}

func (v *IntsDiagramVisitor) generateComponentView(args *Args, viewParams viewParams, params *IntsParam) string {

	fmt.Fprintln(v.sb, "@startuml")
	if viewParams.diagramTitle != "" {
		fmt.Fprintln(v.sb, "title "+viewParams.diagramTitle)
	}
	fmt.Fprintln(v.sb, "hide stereotype")
	fmt.Fprintln(v.sb, "scale max 16384 height")
	fmt.Fprintln(v.sb, "skinparam component {")
	fmt.Fprintln(v.sb, "  BackgroundColor FloralWhite")
	fmt.Fprintln(v.sb, "  BorderColor Black")
	fmt.Fprintln(v.sb, "  ArrowColor Crimson")
	if viewParams.highLightColor != "" {
		fmt.Fprintln(v.sb, "  BackgroundColor<<highlight>> "+viewParams.highLightColor)
	}
	if viewParams.arrowColor != "" {
		fmt.Fprintln(v.sb, "  ArrowColor "+viewParams.arrowColor)
	}

	if viewParams.indirectArrowColor != "" && viewParams.indirectArrowColor != "none" {
		fmt.Fprintln(v.sb, "  ArrowColor<<indirect>> "+viewParams.indirectArrowColor)
	}
	fmt.Fprintln(v.sb, "}")

	nameMap := map[string]string{}
	if args.clustered || viewParams.endptAttrs["view"].GetS() == "clustered" {
		nameMap = v.buildClusterForComponentView(params.apps)
	}
	v.drawComponentView(viewParams, params, nameMap)
	fmt.Fprintln(v.sb, "@enduml")
	return v.sb.String()
}

func GenerateView(args *Args, params *IntsParam, mod *sysl.Module) string {
	var sb strings.Builder
	v := MakeIntsDiagramVisitor(mod, &sb, params.highlights, args.project)
	restrictBy := ""
	if params.endpt.Attrs["restrict_by"] != nil {
		restrictBy = params.endpt.Attrs["restrict_by"].GetS()
	}

	appAttrs := params.app.Attrs
	endptAttrs := params.endpt.Attrs
	highLightColor := appAttrs["highlight_color"].GetS()
	arrowColor := appAttrs["arrow_color"].GetS()
	indirectArrowColor := appAttrs["indirect_arrow_color"].GetS()

	diagramTitle := ""
	attrs := map[string]string{
		"epname":     params.endpt.Name,
		"eplongname": params.endpt.LongName,
	}
	if appAttrs["title"].GetS() != "" {
		diagramTitle = utils.ParseFmt(attrs, appAttrs["title"].GetS())
	} else {
		diagramTitle = utils.ParseFmt(attrs, args.title)
	}

	viewParams := &viewParams{
		restrictBy:         restrictBy,
		endptAttrs:         endptAttrs,
		highLightColor:     highLightColor,
		arrowColor:         arrowColor,
		indirectArrowColor: indirectArrowColor,
		diagramTitle:       diagramTitle,
	}

	fmt.Fprintln(&sb, "''''''''''''''''''''''''''''''''''''''''''")
	fmt.Fprintln(&sb, "''                                      ''")
	fmt.Fprintln(&sb, "''  AUTOGENERATED CODE -- DO NOT EDIT!  ''")
	fmt.Fprintln(&sb, "''                                      ''")
	fmt.Fprintln(&sb, "''''''''''''''''''''''''''''''''''''''''''")
	fmt.Fprintln(&sb)

	if args.epa || endptAttrs["view"].GetS() == "epa" {
		return v.generateStateView(args, *viewParams, params)
	} else {
		return v.generateComponentView(args, *viewParams, params)
	}
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func getApplicationAttrs(m *sysl.Module, appName string) map[string]string {
	val := map[string]string{}
	if app, ok := m.Apps[appName]; ok {
		attrs := app.Attrs
		for k, v := range attrs {
			val["@"+k] = v.GetS()
		}
		return val
	}
	return val
}

func (stmts *Statements) makeStmts(statements []*sysl.Statement) {
	for _, stmt := range statements {
		switch c := stmt.GetStmt().(type) {
		case *sysl.Statement_Call:
			stmts.stmtSlice = append(stmts.stmtSlice, stmt)
		case *sysl.Statement_Action:
			continue
		case *sysl.Statement_Cond:
			stmts.makeStmts(c.Cond.GetStmt())
		case *sysl.Statement_Loop:
			stmts.makeStmts(c.Loop.GetStmt())
		case *sysl.Statement_LoopN:
			stmts.makeStmts(c.LoopN.GetStmt())
		case *sysl.Statement_Foreach:
			stmts.makeStmts(c.Foreach.GetStmt())
		case *sysl.Statement_Group:
			stmts.makeStmts(c.Group.GetStmt())
		case *sysl.Statement_Alt:
			for _, choice := range c.Alt.GetChoice() {
				stmts.makeStmts(choice.GetStmt())
			}
		case *sysl.Statement_Ret:
			continue
		default:
			panic("No statement!")
		}
	}
}

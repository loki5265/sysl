package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"anz-bank/sysl/src/proto"
	"anz-bank/sysl/sysl2/antlr/grammar"
	"github.com/antlr/antlr4/runtime/Go/antlr"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
)

var (
	root   = flag.String("root", ".", "sysl root directory for input files (default: .)")
	output = flag.String("o", "", "output file name")
)

const (
	// ParserSuccess is returned by parser when it was able to parse input correctly
	ParserSuccess = 0
	// ImportError is returned by parser when its unable to load input modules
	ImportError = 1
	// ParseError is returned by parser when one of the input files has syntax errors
	ParseError = 2
)

func init() {
	flag.Parse()
}

// JsonPB ...
func JsonPB(m *sysl.Module, filename string) bool {
	ma := jsonpb.Marshaler{Indent: " ", EmitDefaults: false}
	f, err := os.Create(filename)
	err = ma.Marshal(f, m)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	return true
}

// TextPB ...
func TextPB(m *sysl.Module, filename string) bool {
	if m == nil {
		fmt.Println("input module is nil")
		return false
	}

	f, err := os.Create(filename)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	err = proto.MarshalText(f, m)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	return true
}

// SyslParserErrorListener ...
type SyslParserErrorListener struct {
	*antlr.DefaultErrorListener
	hasErrors bool
}

// SyntaxError ...
func (d *SyslParserErrorListener) SyntaxError(
	recognizer antlr.Recognizer, offendingSymbol interface{},
	line, column int, msg string, e antlr.RecognitionException) {
	d.hasErrors = true
	fmt.Printf("SyntaxError: Token: %s\n", recognizer.GetSymbolicNames()[offendingSymbol.(*antlr.CommonToken).GetTokenType()])
}

// ReportAttemptingFullContext ...
func (d *SyslParserErrorListener) ReportAttemptingFullContext(recognizer antlr.Parser,
	dfa *antlr.DFA, startIndex, stopIndex int,
	conflictingAlts *antlr.BitSet, configs antlr.ATNConfigSet) {
	fmt.Printf("ReportAttemptingFullContext: %d %d\n", startIndex, stopIndex)
}

// ReportAmbiguity ...
func (d *SyslParserErrorListener) ReportAmbiguity(recognizer antlr.Parser,
	dfa *antlr.DFA, startIndex, stopIndex int, exact bool,
	ambigAlts *antlr.BitSet, configs antlr.ATNConfigSet) {
	fmt.Printf("ReportAmbiguity: %d %d\n", startIndex, stopIndex)
}

// ReportContextSensitivity ...
func (d *SyslParserErrorListener) ReportContextSensitivity(recognizer antlr.Parser,
	dfa *antlr.DFA, startIndex, stopIndex, prediction int, configs antlr.ATNConfigSet) {
	fmt.Printf("ReportContextSensitivity: %d %d\n", startIndex, stopIndex)
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

// apply attributes from src to dst statement and all of its
// child statements as well (e.g. For / Loop statements).
func applyAttributes(src *sysl.Statement, dst *sysl.Statement) bool {
	var stmts []*sysl.Statement
	applied := false
	switch s := dst.GetStmt().(type) {
	case *sysl.Statement_Cond:
		stmts = s.Cond.Stmt
	case *sysl.Statement_Alt:
		for _, c := range s.Alt.Choice {
			for _, ss := range c.Stmt {
				applied = applyAttributes(src, ss) || applied
			}
		}
		return applied
	case *sysl.Statement_Group:
		stmts = s.Group.Stmt
	case *sysl.Statement_Loop:
		stmts = s.Loop.Stmt
	case *sysl.Statement_LoopN:
		stmts = s.LoopN.Stmt
	case *sysl.Statement_Foreach:
		stmts = s.Foreach.Stmt
	case *sysl.Statement_Call:
		if isSameCall(src.GetCall(), s.Call) {
			if dst.Attrs == nil {
				dst.Attrs = map[string]*sysl.Attribute{}
			}
			mergeAttrs(src.Attrs, dst.Attrs)
			applied = true
		}
		return applied
	case *sysl.Statement_Action:
		return applied
	case *sysl.Statement_Ret:
		return applied
	default:
		panic("collector: unhandled type")
	}

	for _, stmt := range stmts {
		applied = applyAttributes(src, stmt) || applied
	}
	return applied
}

func checkCalls(mod *sysl.Module, appname string, epname string, dst *sysl.Statement) bool {
	var stmts []*sysl.Statement
	switch s := dst.GetStmt().(type) {
	case *sysl.Statement_Cond:
		stmts = s.Cond.Stmt
	case *sysl.Statement_Alt:
		for _, c := range s.Alt.Choice {
			for _, ss := range c.Stmt {
				if !checkCalls(mod, appname, epname, ss) {
					return false
				}
			}
		}
		return true
	case *sysl.Statement_Group:
		stmts = s.Group.Stmt
	case *sysl.Statement_Loop:
		stmts = s.Loop.Stmt
	case *sysl.Statement_LoopN:
		stmts = s.LoopN.Stmt
	case *sysl.Statement_Foreach:
		stmts = s.Foreach.Stmt
	case *sysl.Statement_Call:
		app := getApp(s.Call.Target, mod)
		if app == nil {
			fmt.Printf("%s::%s calls non-existant App: %s\n",
				appname, epname, s.Call.Target.Part)
			return false
		}
		_, valid := app.Endpoints[s.Call.Endpoint]
		if !valid {
			fmt.Printf("%s::%s calls non-existant App <- Endpoint (%s <- %s)\n",
				appname, epname, s.Call.Target.Part, s.Call.Endpoint)
		}
		return valid
	case *sysl.Statement_Action:
		return true
	case *sysl.Statement_Ret:
		return true
	default:
		panic("collector: unhandled type")
	}

	for _, stmt := range stmts {
		if !checkCalls(mod, appname, epname, stmt) {
			return false
		}
	}
	return true
}

func collectorPubSubCalls(appName string, app *sysl.Application) {
	endpoint := app.Endpoints[`.. * <- *`]
	if endpoint == nil {
		return
	}

	for _, collector_stmt := range endpoint.Stmt {
		switch x := collector_stmt.Stmt.(type) {
		case *sysl.Statement_Action:
			modify_ep := app.Endpoints[x.Action.Action]
			if modify_ep == nil {
				fmt.Printf("App (%s) calls non-existant endpoint (%s)\n",
					appName, x.Action.Action)
				continue
			}
			if modify_ep.Attrs == nil {
				modify_ep.Attrs = map[string]*sysl.Attribute{}
			}
			mergeAttrs(collector_stmt.Attrs, modify_ep.Attrs)
		case *sysl.Statement_Call:
			applied := false

			for call_epname, call_endpoint := range app.Endpoints {
				if call_epname == `.. * <- *` {
					continue
				}
				for _, call_stmt := range call_endpoint.Stmt {
					applied = applyAttributes(collector_stmt, call_stmt) || applied
				}
			}
			if !applied {
				fmt.Printf("Unused template (%s <- %s) in app %s\n",
					x.Call.Target.Part, x.Call.Endpoint, appName)
			}
		default:
			panic("unhandled type:")
		}
	}
}

func checkEndpointCalls(mod *sysl.Module) bool {
	valid := false
	for appName, app := range mod.Apps {
		for epname, ep := range app.Endpoints {
			for _, stmt := range ep.Stmt {
				valid = checkCalls(mod, appName, epname, stmt)
				if !valid {
					return valid
				}
			}
		}
	}
	return valid
}

// for nested transform's Type
func infer_expr_type(mod *sysl.Module,
	appName string,
	expr *sysl.Expr, top bool,
	anonCount int) (*sysl.Type, int) {

	if expr.GetTransform() != nil {
		for _, stmt := range expr.GetTransform().Stmt {
			if stmt.GetLet() != nil {
				_, anonCount = infer_expr_type(mod, appName, stmt.GetLet().Expr, false, anonCount)
			} else if stmt.GetAssign() != nil {
				_, anonCount = infer_expr_type(mod, appName, stmt.GetAssign().Expr, false, anonCount)
			}
		}

		if !top && expr.Type == nil {
			// fmt.Printf("found anonymous type\n")
			newType := &sysl.Type{
				Type: &sysl.Type_Tuple_{
					Tuple: &sysl.Type_Tuple{
						AttrDefs: map[string]*sysl.Type{},
					},
				},
			}
			typeName := fmt.Sprintf("AnonType_%d__", anonCount)
			anonCount++
			if mod.Apps[appName].Types == nil {
				mod.Apps[appName].Types = map[string]*sysl.Type{}
			}
			mod.Apps[appName].Types[typeName] = newType
			attr := newType.GetTuple().AttrDefs
			for _, stmt := range expr.GetTransform().Stmt {
				if stmt.GetAssign() != nil {
					assign := stmt.GetAssign()
					aexpr := assign.Expr
					if aexpr.GetTransform() == nil {
						panic("expression should be of type transform")
					}
					ftype := aexpr.Type
					setof := ftype.GetSet() != nil
					if setof {
						ftype = ftype.GetSet()
					}
					if ftype.GetTypeRef() == nil {
						panic("transform type should be type_ref")
					}
					t1 := &sysl.Type{
						Type: &sysl.Type_TypeRef{
							TypeRef: &sysl.ScopedRef{
								Context: &sysl.Scope{
									Appname: mod.Apps[appName].Name,
									Path:    []string{typeName},
								},
								Ref: ftype.GetTypeRef().Ref,
							},
						},
					}
					if setof {
						t1 = &sysl.Type{
							Type: &sysl.Type_Set{
								Set: t1,
							},
						}
					}
					attr[assign.Name] = t1
				}
			}
			expr.Type = &sysl.Type{
				Type: &sysl.Type_Set{
					Set: &sysl.Type{
						Type: &sysl.Type_TypeRef{
							TypeRef: &sysl.ScopedRef{
								Context: &sysl.Scope{
									Appname: mod.Apps[appName].Name,
								},
								Ref: &sysl.Scope{
									Appname: mod.Apps[appName].Name,
									Path:    []string{typeName},
								},
							},
						},
					},
				},
			}
		}
	} else if expr.GetRelexpr() != nil {
		relexpr := expr.GetRelexpr()
		if relexpr.Op == sysl.Expr_RelExpr_RANK {
			if !top && expr.Type == nil {
				type1, c := infer_expr_type(mod, appName, relexpr.Target, true, anonCount)
				anonCount = c
				fmt.Printf(type1.String())
			}
		}
	}
	return expr.Type, anonCount
}

func infer_types(mod *sysl.Module, appName string) {
	for viewName, view := range mod.Apps[appName].Views {
		if hasAbstractPattern(view.Attrs) {
			continue
		}
		if view.Expr.GetTransform() == nil {
			fmt.Printf("view %s expression should be of type transform", viewName)
			continue
		}
		infer_expr_type(mod, appName, view.Expr, true, 0)
	}
}

func postProcess(mod *sysl.Module) {
	var appNames []string
	for a := range mod.Apps {
		appNames = append(appNames, a)
	}
	sort.Strings(appNames)

	for _, appName := range appNames {
		app := mod.Apps[appName]

		if app.Mixin2 != nil {
			for _, src := range app.Mixin2 {
				src_app := getApp(src.Name, mod)
				if hasAbstractPattern(src_app.Attrs) == false {
					fmt.Printf("mixin App (%s) should be ~abstract\n", getAppName(src.Name))
					continue
				}
				if src_app.Types != nil && app.Types == nil {
					app.Types = map[string]*sysl.Type{}
				}
				if src_app.Views != nil && app.Views == nil {
					app.Views = map[string]*sysl.View{}
				}
				for k, v := range src_app.Types {
					if _, has := app.Types[k]; !has {
						app.Types[k] = v
					} else {
						fmt.Printf("Type %s defined in %s and in %s\n",
							k, appName, getAppName(src.Name))
					}
				}
				for k, v := range src_app.Views {
					if _, has := app.Views[k]; !has {
						app.Views[k] = v
					} else {
						fmt.Printf("View %s defined in %s and in %s\n",
							k, appName, getAppName(src.Name))
					}
				}
			}
		}

		for typeName, types := range app.Types {
			var attrs map[string]*sysl.Type

			switch x := types.Type.(type) {
			case *sysl.Type_Tuple_:
				attrs = x.Tuple.GetAttrDefs()
			case *sysl.Type_Relation_:
				attrs = x.Relation.GetAttrDefs()
			}
			for fieldname, t := range attrs {
				if x := t.GetTypeRef(); x != nil {
					refApp := app
					var refName string
					refName = x.GetRef().GetPath()[0]
					if refName == "string_8" {
						continue
					}
					refType, has := refApp.Types[refName]
					if has == false {
						fmt.Printf("1:Field %s (type %s) refers to type (%s) in app (%s)\n",
							fieldname, typeName, refName, appName)
					} else {
						var ref_attrs map[string]*sysl.Type

						switch refType.Type.(type) {
						case *sysl.Type_Tuple_:
							refType, _ := refApp.Types[refName].Type.(*sysl.Type_Tuple_)
							ref_attrs = refType.Tuple.GetAttrDefs()
						case *sysl.Type_Relation_:
							refType, _ := refApp.Types[refName].Type.(*sysl.Type_Relation_)
							ref_attrs = refType.Relation.GetAttrDefs()
						}
						var field string
						var has bool
						if len(x.GetRef().GetPath()) > 1 {
							last := len(x.GetRef().GetPath()) - 1
							field = x.GetRef().GetPath()[last]
							_, has = ref_attrs[field]
						} else if len(x.GetRef().GetPath()) == 1 {
							last := len(x.GetRef().GetPath()) - 1
							field = x.GetRef().GetPath()[last]
							_, has = refApp.Types[field]
						}
						if has == false {
							fmt.Printf("2:Field %s (type %s) refers to Field (%s) in app (%s)/type (%s)\n",
								fieldname, typeName, field, appName, refName)
						}
					}
				}
			}
		}
		infer_types(mod, appName)
		collectorPubSubCalls(appName, app)
	}
	checkEndpointCalls(mod)
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func dirExists(dirName string) bool {
	info, err := os.Stat(dirName)
	return err == nil && info.IsDir()
}

// Parse ...
func Parse(filename string, root string) *sysl.Module {
	if root == "" {
		root = "."
	}
	if !dirExists(root) {
		fmt.Fprintln(os.Stderr, "root directory does not exist")
		os.Exit(ImportError)
	}
	root, _ = filepath.Abs(root)

	if !fileExists(filename) {
		if !strings.HasSuffix(filename, ".sysl") {
			filename = filename + ".sysl"
		}
		filename = root + "/" + filename

		if !fileExists(filename) {
			fmt.Fprintln(os.Stderr, fmt.Sprintf("input file does not exist\nFilename: %s\n", filename))
			os.Exit(ImportError)
		}
	}
	filename, _ = filepath.Abs(filename)
	imported := map[string]struct{}{}
	listener := NewTreeShapeListener(root)
	errorListener := SyslParserErrorListener{}

	for {
		fmt.Println(filename)
		input, err := antlr.NewFileStream(filename)
		if err != nil {
			fmt.Fprintln(os.Stderr, fmt.Sprintf("%v,\n%s has errors\n", err, filename))
			os.Exit(ImportError)
		}
		listener.filename = filename
		listener.base = filepath.Dir(filename)
		lexer := parser.NewSyslLexer(input)
		stream := antlr.NewCommonTokenStream(lexer, 0)
		p := parser.NewSyslParser(stream)
		p.GetInterpreter().SetPredictionMode(antlr.PredictionModeSLL)
		p.AddErrorListener(antlr.NewDiagnosticErrorListener(true))
		p.AddErrorListener(&errorListener)

		p.BuildParseTrees = true
		tree := p.Sysl_file()
		if errorListener.hasErrors {
			fmt.Fprintln(os.Stderr, fmt.Sprintf("%s has syntax errors\n", filename))
			os.Exit(ParseError)
		}

		antlr.ParseTreeWalkerDefault.Walk(listener, tree)
		if len(listener.imports) == 0 {
			break
		}
		imported[filename] = struct{}{}

		for len(listener.imports) > 0 {
			filename = listener.imports[0]
			listener.imports = listener.imports[1:]
			if _, has := imported[filename]; !has {
				break
			}
		}
		if _, has := imported[filename]; has {
			break
		}
	}

	postProcess(listener.module)
	return listener.module
}

func main() {
	fmt.Printf("Args: %v\n", flag.Args())
	fmt.Printf("Root: %s\n", *root)
	fmt.Printf("Module: %s\n", flag.Arg(0))
	format := strings.ToLower(*output)
	toJson := strings.HasSuffix(format, ".json")
	mod := Parse(flag.Arg(0), *root)
	if mod != nil {
		if toJson {
			JsonPB(mod, *output)
		} else {
			TextPB(mod, *output)
		}
	}
}
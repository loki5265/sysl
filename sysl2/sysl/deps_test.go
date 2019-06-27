package main

import (
	"testing"

	sysl "github.com/anz-bank/sysl/src/proto"
	"github.com/stretchr/testify/assert"
)

func TestAppDependency_String(t *testing.T) {
	// Given
	dep := MakeAppDependency(MakeAppElement("AppA", "EndptA"), MakeAppElement("AppB", "EndptB"))
	expected := "s.name: AppA, s.end: EndptA, t.name: AppB, t.end: EndptB"

	// When
	actual := dep.String()

	// Then
	assert.Equal(t, actual, expected)
}

func TestCollectCallStatements(t *testing.T) {
	// Given
	stmts := []*sysl.Statement{
		{
			Stmt: &sysl.Statement_Call{
				Call: &sysl.Call{
					Target: &sysl.AppName{
						Part: []string{"AppA"},
					},
					Endpoint: "EndptA",
				},
			},
		},
		{
			Stmt: &sysl.Statement_Action{
				Action: &sysl.Action{
					Action: "Get",
				},
			},
		},
		{
			Stmt: &sysl.Statement_Ret{
				Ret: &sysl.Return{
					Payload: "Return A",
				},
			},
		},
		{
			Stmt: &sysl.Statement_Cond{
				Cond: &sysl.Cond{
					Stmt: []*sysl.Statement{
						{
							Stmt: &sysl.Statement_Call{
								Call: &sysl.Call{
									Target: &sysl.AppName{
										Part: []string{"AppB"},
									},
									Endpoint: "EndptB",
								},
							},
						},
					},
				},
			},
		},
		{
			Stmt: &sysl.Statement_Loop{
				Loop: &sysl.Loop{
					Stmt: []*sysl.Statement{},
				},
			},
		},
		{
			Stmt: &sysl.Statement_LoopN{
				LoopN: &sysl.LoopN{
					Stmt: []*sysl.Statement{},
				},
			},
		},
		{
			Stmt: &sysl.Statement_Foreach{
				Foreach: &sysl.Foreach{
					Stmt: []*sysl.Statement{},
				},
			},
		},
		{
			Stmt: &sysl.Statement_Group{
				Group: &sysl.Group{
					Stmt: []*sysl.Statement{},
				},
			},
		},
		{
			Stmt: &sysl.Statement_Alt{
				Alt: &sysl.Alt{
					Choice: []*sysl.Alt_Choice{
						{
							Stmt: []*sysl.Statement{},
						},
					},
				},
			},
		},
	}
	cs := NewCallSlice()
	expected := []*sysl.Call{
		{
			Target: &sysl.AppName{
				Part: []string{"AppA"},
			},
			Endpoint: "EndptA",
		},
		{
			Target: &sysl.AppName{
				Part: []string{"AppB"},
			},
			Endpoint: "EndptB",
		},
	}

	// When
	cs.CollectCallStatements(stmts)

	// Then
	assert.Equal(t, cs.GetSlice(), expected)
}

var mod = &sysl.Module{
	Apps: map[string]*sysl.Application{
		"AppA": {
			Attrs: map[string]*sysl.Attribute{},
		},
		"AppB": {
			Attrs: map[string]*sysl.Attribute{},
			Endpoints: map[string]*sysl.Endpoint{
				"EndptB": {
					Stmt: []*sysl.Statement{
						{
							Stmt: &sysl.Statement_Call{
								Call: &sysl.Call{
									Target: &sysl.AppName{
										Part: []string{"AppE"},
									},
									Endpoint: "EndptE",
								},
							},
						},
						{
							Stmt: &sysl.Statement_Action{
								Action: &sysl.Action{
									Action: "Get",
								},
							},
						},
						{
							Stmt: &sysl.Statement_Ret{
								Ret: &sysl.Return{
									Payload: "Return A",
								},
							},
						},
						{
							Stmt: &sysl.Statement_Cond{
								Cond: &sysl.Cond{
									Stmt: []*sysl.Statement{},
								},
							},
						},
						{
							Stmt: &sysl.Statement_Loop{
								Loop: &sysl.Loop{
									Stmt: []*sysl.Statement{},
								},
							},
						},
						{
							Stmt: &sysl.Statement_LoopN{
								LoopN: &sysl.LoopN{
									Stmt: []*sysl.Statement{},
								},
							},
						},
						{
							Stmt: &sysl.Statement_Foreach{
								Foreach: &sysl.Foreach{
									Stmt: []*sysl.Statement{},
								},
							},
						},
						{
							Stmt: &sysl.Statement_Group{
								Group: &sysl.Group{
									Stmt: []*sysl.Statement{},
								},
							},
						},
						{
							Stmt: &sysl.Statement_Alt{
								Alt: &sysl.Alt{
									Choice: []*sysl.Alt_Choice{
										{
											Stmt: []*sysl.Statement{},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"AppC": {
			Attrs: map[string]*sysl.Attribute{},
		},
		"AppD": {
			Attrs: map[string]*sysl.Attribute{},
		},
		"AppE": {
			Attrs: map[string]*sysl.Attribute{},
			Endpoints: map[string]*sysl.Endpoint{
				"EndptE": {
					Stmt: []*sysl.Statement{},
				},
			},
		},
	},
}

func TestFindHighlightApps(t *testing.T) {
	// Given
	ds := NewDependencySet()
	dep := MakeAppDependency(MakeAppElement("AppA", "EndptA"), MakeAppElement("AppB", "EndptB"))
	ds.Add(dep)

	dep1 := MakeAppDependency(MakeAppElement("AppC", "EndptC"), MakeAppElement("AppD", "EndptD"))
	ds.Add(dep1)

	excludes := MakeStrSet("AppC", "AppD")
	integrations := MakeStrSet("AppA", "AppB")
	expected := MakeStrSet("AppA", "AppB")

	// When
	actual := FindApps(mod, excludes, integrations, ds, true)

	// Then
	assert.Equal(t, actual, expected)
}

func TestFindNoneHighlightApps(t *testing.T) {
	// Given
	ds := NewDependencySet()
	dep := MakeAppDependency(MakeAppElement("AppA", "EndptA"), MakeAppElement("AppB", "EndptB"))
	ds.Add(dep)

	dep1 := MakeAppDependency(MakeAppElement("AppC", "EndptC"), MakeAppElement("AppD", "EndptD"))
	ds.Add(dep1)

	excludes := MakeStrSet("AppC", "AppD")
	integrations := MakeStrSet("AppA", "AppB")
	expected := MakeStrSet("AppA", "AppB")

	// When
	actual := FindApps(mod, excludes, integrations, ds, false)

	// Then
	assert.Equal(t, actual, expected)
}

func TestNotFindNoneHighlightApps(t *testing.T) {
	// Given
	ds := NewDependencySet()
	dep := MakeAppDependency(MakeAppElement("AppA", "EndptA"), MakeAppElement("AppB", "EndptB"))
	ds.Add(dep)

	dep1 := MakeAppDependency(MakeAppElement("AppC", "EndptC"), MakeAppElement("AppD", "EndptD"))
	ds.Add(dep1)

	excludes := MakeStrSet("AppC", "AppD")
	integrations := MakeStrSet("AppE", "AppF")
	expected := MakeStrSet()

	// When
	actual := FindApps(mod, excludes, integrations, ds, false)

	// Then
	assert.Equal(t, actual, expected)
}

func TestFindIntegrations(t *testing.T) {
	// Given
	ds := NewDependencySet()
	expected := NewDependencySet()
	tar := MakeAppElement("AppB", "EndptB")
	dep := MakeAppDependency(MakeAppElement("AppA", "EndptA"), tar)
	ds.Add(dep)

	dep1 := MakeAppDependency(MakeAppElement("AppC", "EndptC"), MakeAppElement("AppD", "EndptD"))
	ds.Add(dep1)

	apps := MakeStrSet("AppA", "AppB")
	excludes := MakeStrSet("AppC")
	passthrough := MakeStrSet("AppB", "AppE", "AppF")

	dep2 := MakeAppDependency(tar, MakeAppElement("AppE", "EndptE"))
	expected.Add(dep)
	expected.Add(dep2)

	// When
	actual := ds.FindIntegrations(apps, excludes, passthrough, mod)

	// Then
	assert.Equal(t, actual.Deps, expected.Deps)
}

func TestDependencySet_ResolveDependencies(t *testing.T) {
	// Given
	ds := NewDependencySet()

	// When
	ds.CollectAppDependencies(mod)

	// Then
	assert.Equal(t, 1, len(ds.Deps))
}

func TestSubWhenParentAndChildEmpty(t *testing.T) {
	// Given
	c := MakeStrSet()
	p := MakeStrSet()
	expected := true

	// When
	actual := isSub(c, p)

	// Then
	assert.Equal(t, expected, actual)
}

func TestSubWhenParentEmpty(t *testing.T) {
	// Given
	c := MakeStrSet("A")
	p := MakeStrSet()
	expected := false

	// When
	actual := isSub(c, p)

	// Then
	assert.Equal(t, expected, actual)
}

func TestSubWhenChildEmpty(t *testing.T) {
	// Given
	c := MakeStrSet()
	p := MakeStrSet("A")
	expected := true

	// When
	actual := isSub(c, p)

	// Then
	assert.Equal(t, expected, actual)
}

func TestBuildStringBoolFilter(t *testing.T) {
	// Given
	i := []string{"a"}
	expected := map[string]bool{
		"a": true,
	}

	// When
	actual := buildStringBoolFilter(i)

	// Then
	assert.Equal(t, expected, actual)
}

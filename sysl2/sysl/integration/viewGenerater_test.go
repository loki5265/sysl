package integration

import (
	"github.com/anz-bank/sysl/src/proto"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestGenerateView(t *testing.T) {
}

func TestVarManagerForComponent(t *testing.T) {
	//Given
	var sb strings.Builder
	v := &IntsDiagramVisitor{
		sb:         &sb,
		m:          &sysl.Module{},
		highlights: map[string]struct{}{},
		symbols:    map[string]*_var{},
	}

	//When
	result := v.VarManagerForComponent("test", map[string]string{})

	//Then
	assert.Equal(t, "_0", result)
}

func TestVarManagerForComponentWithNameMap(t *testing.T) {
	//Given
	var sb strings.Builder
	v := &IntsDiagramVisitor{
		sb:         &sb,
		m:          &sysl.Module{},
		highlights: map[string]struct{}{},
		symbols: map[string]*_var{
			"appName": &_var{
				alias: "_1",
			},
		},
	}

	//When
	result := v.VarManagerForComponent("test", map[string]string{
		"test": "appName",
	})

	//Then
	assert.Equal(t, "_1", result)
}

func TestVarManagerForComponentWithExistingName(t *testing.T) {
	//Given
	var sb strings.Builder
	v := &IntsDiagramVisitor{
		sb:         &sb,
		m:          &sysl.Module{},
		highlights: map[string]struct{}{},
		symbols: map[string]*_var{
			"test": &_var{
				alias: "_1",
			},
		},
	}

	//When
	result := v.VarManagerForComponent("test", map[string]string{})

	//Then
	assert.Equal(t, "_1", result)
}

func TestVarManagerForState(t *testing.T) {
	//Given
	var sb strings.Builder
	v := &IntsDiagramVisitor{
		sb: &sb,
		m: &sysl.Module{
			Apps: map[string]*sysl.Application{
				"a": &sysl.Application{
					Endpoints: map[string]*sysl.Endpoint{
						"b": nil,
					},
				},
			},
		},
		highlights: map[string]struct{}{},
		symbols:    map[string]*_var{},
	}

	//When
	result := v.VarManagerForState("a : b")

	//Then
	assert.Equal(t, "_0", result)
}

func TestVarManagerForStateWithExistingName(t *testing.T) {
	//Given
	var sb strings.Builder
	v := &IntsDiagramVisitor{
		sb: &sb,
		m: &sysl.Module{
			Apps: map[string]*sysl.Application{
				"a": &sysl.Application{
					Endpoints: map[string]*sysl.Endpoint{
						"b": &sysl.Endpoint{
							Attrs: map[string]*sysl.Attribute{
								"test": nil,
							},
						},
					},
				},
			},
		},
		highlights: map[string]struct{}{},
		symbols: map[string]*_var{
			"a : b": &_var{
				alias: "_1",
			},
		},
	}

	//When
	result := v.VarManagerForState("a : b")

	//Then
	assert.Equal(t, "_1", result)
}

func TestVarManagerForTopState(t *testing.T) {
	//Given
	var sb strings.Builder
	v := &IntsDiagramVisitor{
		sb:         &sb,
		m:          &sysl.Module{},
		highlights: map[string]struct{}{},
		topSymbols: map[string]*_topVar{},
	}

	//When
	result := v.VarManagerForTopState("a : b")

	//Then
	assert.Equal(t, "_0", result)
}

func TestVarManagerForTopStateWithExistingName(t *testing.T) {
	//Given
	var sb strings.Builder
	v := &IntsDiagramVisitor{
		sb:         &sb,
		m:          &sysl.Module{},
		highlights: map[string]struct{}{},
		topSymbols: map[string]*_topVar{
			"a : b": &_topVar{
				topAlias: "_1",
			},
		},
	}

	//When
	result := v.VarManagerForTopState("a : b")

	//Then
	assert.Equal(t, "_1", result)
}

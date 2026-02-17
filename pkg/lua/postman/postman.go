package postman

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// Module provides Lua bindings for running Postman collections via Newman.
type Module struct{}

// New creates a new Postman module.
func New() *Module {
	return &Module{}
}

// Loader registers the postman module functions into the Lua state.
func (m *Module) Loader(L *lua.LState) int {
	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"run": m.Run,
	})
	L.Push(mod)
	return 1
}

// Run executes a Postman collection using Newman.
// Lua usage:
//
//	local passed, output = pm.run({
//	    collection  = "tests/api.json",
//	    environment = "tests/env.json",   -- optional
//	    folder      = "Login Flow",       -- optional
//	    reporters   = "cli,json",         -- optional
//	    data        = "tests/data.csv",   -- optional
//	    env_vars    = { key = "value" },  -- optional
//	})
func (m *Module) Run(L *lua.LState) int {
	opts := L.CheckTable(1)

	collection := opts.RawGetString("collection")
	if collection == lua.LNil || collection.String() == "" {
		L.RaiseError("postman.run: 'collection' is required")
		return 0
	}

	// Build newman command args
	args := []string{"run", collection.String()}

	// Environment file
	if env := opts.RawGetString("environment"); env != lua.LNil && env.String() != "" {
		args = append(args, "-e", env.String())
	}

	// Folder filter
	if folder := opts.RawGetString("folder"); folder != lua.LNil && folder.String() != "" {
		args = append(args, "--folder", folder.String())
	}

	// Reporters
	if reporters := opts.RawGetString("reporters"); reporters != lua.LNil && reporters.String() != "" {
		args = append(args, "--reporters", reporters.String())
	}

	// Data file (CSV/JSON for data-driven tests)
	if data := opts.RawGetString("data"); data != lua.LNil && data.String() != "" {
		args = append(args, "-d", data.String())
	}

	// Environment variables (key=value pairs)
	if envVars := opts.RawGetString("env_vars"); envVars != lua.LNil && envVars.Type() == lua.LTTable {
		envVars.(*lua.LTable).ForEach(func(k, v lua.LValue) {
			args = append(args, "--env-var", fmt.Sprintf("%s=%s", k.String(), v.String()))
		})
	}

	// Execute newman
	fmt.Printf("[postman] Running: newman %s\n", strings.Join(args, " "))

	cmd := exec.Command("newman", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()

	passed := err == nil

	// Return: passed (bool), output summary (string)
	L.Push(lua.LBool(passed))
	if err != nil {
		L.Push(lua.LString(fmt.Sprintf("newman failed: %v", err)))
	} else {
		L.Push(lua.LString("all tests passed"))
	}
	return 2
}

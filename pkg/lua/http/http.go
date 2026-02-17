package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/chakradharkondapalli/topas/pkg/lua/util"
	lua "github.com/yuin/gopher-lua"
)

type Module struct{}

func New() *Module {
	return &Module{}
}

func (m *Module) Loader(L *lua.LState) int {
	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"expect": m.Expect,
	})
	L.Push(mod)
	return 1
}

func (m *Module) Expect(L *lua.LState) int {
	reqTable := L.CheckTable(1)

	// 1. Parse Request
	url := reqTable.RawGetString("url").String()
	method := reqTable.RawGetString("method").String()
	if method == "" {
		method = "GET"
	}

	var bodyReader io.Reader
	bodyVal := reqTable.RawGetString("body")
	if bodyVal.Type() != lua.LTNil {
		// Serialize Lua table to JSON
		jsonBody, err := toJSON(bodyVal)
		if err != nil {
			L.RaiseError("failed to serialize body: %v", err)
			return 0
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		L.RaiseError("failed to create request: %v", err)
		return 0
	}
	req.Header.Set("Content-Type", "application/json")

	// 2. Execute Request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		L.RaiseError("request failed: %v", err)
		return 0
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		L.RaiseError("failed to read response: %v", err)
		return 0
	}

	// 3. Assertions
	expectVal := reqTable.RawGetString("expect")
	if expectVal.Type() == lua.LTTable {
		expectTable := expectVal.(*lua.LTable)

		// Assert Status
		statusVal := expectTable.RawGetString("status")
		if statusVal.Type() == lua.LTNumber {
			expectedStatus := int(statusVal.(lua.LNumber))
			if resp.StatusCode != expectedStatus {
				L.RaiseError("assertion failed: expected status %d, got %d. Body: %s", expectedStatus, resp.StatusCode, string(respBody))
				return 0
			}
		}

		// Assert Body (Subset Match)
		expectBodyVal := expectTable.RawGetString("body")
		if expectBodyVal.Type() != lua.LTNil {
			var actual interface{}
			if err := json.Unmarshal(respBody, &actual); err != nil {
				L.RaiseError("failed to parse response body as JSON: %v. Body: %s", err, string(respBody))
				return 0
			}

			expectedJSON, _ := toJSON(expectBodyVal)
			var expected interface{}
			json.Unmarshal(expectedJSON, &expected)

			if !subsetMatch(expected, actual) {
				L.RaiseError("assertion failed: body mismatch. Expected subset %s, got %s", string(expectedJSON), string(respBody))
				return 0
			}
		}
	}

	return 0
}

// Helper: Convert Lua value to JSON bytes
func toJSON(v lua.LValue) ([]byte, error) {
	return util.ToJSON(v)
}

func toGoValue(v lua.LValue) interface{} {
	return util.ToGoValue(v)
}

// subsetMatch checks if 'expected' is a subset of 'actual'
func subsetMatch(expected, actual interface{}) bool {
	switch exp := expected.(type) {
	case map[string]interface{}:
		act, ok := actual.(map[string]interface{})
		if !ok {
			return false
		}
		for k, v := range exp {
			if !subsetMatch(v, act[k]) {
				return false
			}
		}
		return true
	case []interface{}:
		act, ok := actual.([]interface{})
		if !ok || len(exp) > len(act) {
			return false
		}
		// Simplified: Array match assumes exact order or subset check?
		// For robustness, let's assume exact match for arrays for now, or implement deeper logic.
		// Implementing exact match for array elements for simplicity.
		for i, v := range exp {
			if !subsetMatch(v, act[i]) {
				return false
			}
		}
		return true
	default:
		return expected == actual
	}
}

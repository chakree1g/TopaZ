package util

import (
	"encoding/json"

	lua "github.com/yuin/gopher-lua"
)

// ToGoValue converts a Lua value to a Go value
func ToGoValue(v lua.LValue) interface{} {
	switch v.Type() {
	case lua.LTNil:
		return nil
	case lua.LTBool:
		return bool(v.(lua.LBool))
	case lua.LTNumber:
		return float64(v.(lua.LNumber))
	case lua.LTString:
		return string(v.(lua.LString))
	case lua.LTTable:
		t := v.(*lua.LTable)
		// Check if it's an array or object
		if t.MaxN() > 0 { // Array
			arr := make([]interface{}, 0, t.MaxN())
			t.ForEach(func(_, val lua.LValue) {
				arr = append(arr, ToGoValue(val))
			})
			return arr
		} else { // Object
			obj := make(map[string]interface{})
			t.ForEach(func(key, val lua.LValue) {
				obj[key.String()] = ToGoValue(val)
			})
			return obj
		}
	default:
		return nil
	}
}

func ToJSON(v lua.LValue) ([]byte, error) {
	data := ToGoValue(v)
	return json.Marshal(data)
}

// ToLuaValue converts a Go value to a Lua value
func ToLuaValue(L *lua.LState, v interface{}) lua.LValue {
	switch v := v.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(v)
	case float64:
		return lua.LNumber(v)
	case string:
		return lua.LString(v)
	case []interface{}:
		t := L.NewTable()
		for i, val := range v {
			t.RawSetInt(i+1, ToLuaValue(L, val))
		}
		return t
	case map[string]interface{}:
		t := L.NewTable()
		for key, val := range v {
			t.RawSetString(key, ToLuaValue(L, val))
		}
		return t
	default:
		return lua.LNil
	}
}

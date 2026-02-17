package net

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/chakradharkondapalli/topas/pkg/lua/util"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/jhump/protoreflect/grpcreflect"
	lua "github.com/yuin/gopher-lua"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Module struct {
	Client *http.Client
}

func New() *Module {
	return &Module{
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (m *Module) Loader(L *lua.LState) int {
	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"request": m.Request,
		"get":     m.Get,
		"post":    m.Post,
		"grpc":    m.Grpc,
	})
	L.Push(mod)
	return 1
}

// HTTP Helpers

func (m *Module) Get(L *lua.LState) int {
	url := L.CheckString(1)
	return m.doRequest(L, "GET", url, nil)
}

func (m *Module) Post(L *lua.LState) int {
	url := L.CheckString(1)
	body := L.CheckAny(2)
	return m.doRequest(L, "POST", url, body)
}

// gRPC Helper
func (m *Module) Grpc(L *lua.LState) int {
	// grpc(address, method, body)
	addr := L.CheckString(1)   // "host:port"
	method := L.CheckString(2) // "Service/Method" or "package.Service/Method"
	body := L.OptTable(3, L.NewTable())

	// Construct grpc:// URL for unified Request handler
	// url = grpc://host:port/Service/Method
	url := fmt.Sprintf("grpc://%s/%s", addr, method)

	// Delegate to Request
	reqTable := L.NewTable()
	reqTable.RawSetString("url", lua.LString(url))
	reqTable.RawSetString("body", body)

	L.Push(reqTable)
	return m.Request(L)
}

// Unified Request Handler
func (m *Module) Request(L *lua.LState) int {
	// request({ url="...", method="...", body=... })
	req := L.CheckTable(1)
	urlStr := req.RawGetString("url").String()

	if strings.HasPrefix(urlStr, "http") {
		method := req.RawGetString("method").String()
		if method == "" {
			method = "GET"
		}
		body := req.RawGetString("body")
		return m.doRequest(L, method, urlStr, body)
	} else if strings.HasPrefix(urlStr, "grpc") {
		body := req.RawGetString("body")
		return m.doGrpcRequest(L, urlStr, body)
	} else {
		L.RaiseError("unsupported scheme in url: %s", urlStr)
		return 0
	}
}

func (m *Module) doRequest(L *lua.LState, method, urlStr string, body lua.LValue) int {
	var bodyReader io.Reader
	if body != nil && body != lua.LNil {
		goVal := util.ToGoValue(body)
		jsonBody, err := json.Marshal(goVal)
		if err != nil {
			L.RaiseError("failed to marshal body: %v", err)
			return 0
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, urlStr, bodyReader)
	if err != nil {
		L.RaiseError("failed to create request: %v", err)
		return 0
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := m.Client.Do(req)
	if err != nil {
		L.RaiseError("request failed: %v", err)
		return 0
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// Return table: { code=200, body="...", json=func() }
	ret := L.NewTable()
	ret.RawSetString("code", lua.LNumber(resp.StatusCode))
	ret.RawSetString("body", lua.LString(string(respBody)))

	// Helper to parse JSON response
	ret.RawSetString("json", L.NewFunction(func(L *lua.LState) int {
		var result interface{}
		if err := json.Unmarshal(respBody, &result); err != nil {
			L.RaiseError("failed to parse json: %v", err)
			return 0
		}
		L.Push(util.ToLuaValue(L, result))
		return 1
	}))

	L.Push(ret)
	return 1
}

func (m *Module) doGrpcRequest(L *lua.LState, urlStr string, body lua.LValue) int {
	// urlStr: grpc://host:port/Service/Method
	// Parse URL
	parts := strings.SplitN(strings.TrimPrefix(urlStr, "grpc://"), "/", 2)
	if len(parts) != 2 {
		L.RaiseError("invalid grpc url format: expected grpc://host:port/Service/Method")
		return 0
	}
	addr := parts[0]
	fullMethod := parts[1] // Service/Method or package.Service/Method

	// Split Service and Method
	methodParts := strings.Split(fullMethod, "/")
	if len(methodParts) != 2 {
		L.RaiseError("invalid method format: expected Service/Method")
		return 0
	}
	serviceName := methodParts[0]
	methodName := methodParts[1]

	ctx := context.Background()

	// Dial gRPC
	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		L.RaiseError("failed to dial grpc: %v", err)
		return 0
	}
	defer conn.Close()

	// Create Reflection Client
	refClient := grpcreflect.NewClientAuto(ctx, conn)
	defer refClient.Reset()

	// Resolve Service
	svcDesc, err := refClient.ResolveService(serviceName)
	if err != nil {
		L.RaiseError("failed to resolve service %s: %v", serviceName, err)
		return 0
	}
	// Find Method
	methodDesc := svcDesc.FindMethodByName(methodName)
	if methodDesc == nil {
		L.RaiseError("method %s not found in service %s", methodName, serviceName)
		return 0
	}

	// Create Dynamic Stub
	stub := grpcdynamic.NewStub(conn)

	// Convert Body (Lua Table) -> JSON -> Dynamic Message
	reqMsg := dynamic.NewMessage(methodDesc.GetInputType())
	if body != lua.LNil {
		goVal := util.ToGoValue(body)
		jsonBody, err := json.Marshal(goVal)
		if err != nil {
			L.RaiseError("failed to marshal lua body to json: %v", err)
			return 0
		}
		if err := reqMsg.UnmarshalJSON(jsonBody); err != nil {
			L.RaiseError("failed to unmarshal json to proto message: %v", err)
			return 0
		}
	}

	// Invoke RPC
	respMsg, err := stub.InvokeRpc(ctx, methodDesc, reqMsg)
	if err != nil {
		L.RaiseError("grpc call failed: %v", err)
		return 0
	}

	// Convert Response (Dynamic Message) -> JSON -> Lua Table
	respJson, err := json.Marshal(respMsg)
	if err != nil {
		L.RaiseError("failed to marshal response to json: %v", err)
		return 0
	}

	// Unmarshal JSON to interface{} for ToLuaValue
	var respObj interface{}
	if len(respJson) > 0 {
		if err := json.Unmarshal(respJson, &respObj); err != nil {
			L.RaiseError("failed to parse response json: %v", err)
			return 0
		}
	}

	L.Push(util.ToLuaValue(L, respObj))
	return 1
}

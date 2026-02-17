local net = require("net")
local sut = require("sut")

-- 1. HTTP Request (GET)
print("Testing HTTP GET...")
local resp = net.get("http://mock-app-echo:8080/api/items")
assert(resp.code == 200, "HTTP GET failed: code " .. resp.code)
-- Check if body is valid JSON array (using helper)
-- local items = resp.json()
-- assert(type(items) == "table", "HTTP GET body is not a table")
print("HTTP GET Passed")

-- 2. HTTP Request (POST)
print("Testing HTTP POST...")
local new_item = {name="net-test", price=99.99}
local resp_post = net.post("http://mock-app-echo:8080/api/items", new_item)
assert(resp_post.code == 201, "HTTP POST failed: code " .. resp_post.code)
print("HTTP POST Passed")

-- 3. gRPC Request (Dynamic Health Check)
print("Testing gRPC Health Check...")
-- scheme: grpc://host:port/Service/Method
-- Health Service: grpc.health.v1.Health/Check
-- Input: { service="" }
local grpc_resp = net.request({
    url = "grpc://mock-app-echo:50051/grpc.health.v1.Health/Check",
    body = { service = "" }
})

-- Expected Output: { status = "SERVING" } or { status = 1 } depending on JSON marshalling
-- Print response for debugging
if grpc_resp.status then
    print("gRPC Health Check Status: " .. tostring(grpc_resp.status))
else
    print("gRPC Health Check Response missing status field")
    -- Primitive dump
    for k,v in pairs(grpc_resp) do print(k,v) end
end

-- Check success (adjust based on logs)
assert(grpc_resp.status == 1 or grpc_resp.status == "SERVING", "gRPC Health Check failed: status " .. tostring(grpc_resp.status))
print("gRPC Health Check Passed")

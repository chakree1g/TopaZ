-- smoke-test.lua
-- Basic smoke test: verify the echo service is reachable and returns expected data.

local http = require("http")

print("=== TOPAS Smoke Test ===")

-- Test 1: Health check
print("[1/3] Checking /health endpoint...")
http.expect({
    url    = "http://mock-app-echo:8080/health",
    method = "GET",
    expect = {
        status = 200,
        body   = { status = "ok" }
    }
})
print("  ✓ Health check passed")

-- Test 2: Echo endpoint
print("[2/3] Checking /api/echo endpoint...")
http.expect({
    url    = "http://mock-app-echo:8080/api/echo?msg=topas",
    method = "GET",
    expect = {
        status = 200,
        body   = { echo = "topas" }
    }
})
print("  ✓ Echo endpoint passed")

-- Test 3: Version endpoint
print("[3/3] Checking /api/version endpoint...")
http.expect({
    url    = "http://mock-app-echo:8080/api/version",
    method = "GET",
    expect = {
        status = 200,
        body   = { version = "v1" }
    }
})
print("  ✓ Version endpoint passed")

print("=== ALL TESTS PASSED ===")

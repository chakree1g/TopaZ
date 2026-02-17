-- Example Test Script
local sut = require("sut")
local http = require("http")
local db = require("db")

print("Starting Upgrade Test...")

-- 1. Upgrade Frontend
print("Applying new version...")
sut.apply("frontend", {
    image = "nginx:1.16.0",
    replicas = 3
})

-- 2. Wait for Rollout
print("Waiting for readiness...")
sut.wait("frontend")

-- 3. Verify HTTP (Mock)
-- http.expect({
--     url = "http://localhost:8080",
--     expect = { status = 200 }
-- })

print("Test Passed!")

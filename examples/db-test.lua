-- db-test.lua
-- Integration test: seed items via db.seed(), verify via db.expect() and http.expect().

local db   = require("db")
local http = require("http")

print("=== TOPAS DB Integration Test ===")

-- 1. Connect to the database directly
print("[1/5] Connecting to database...")
db.connect({
    host     = "mock-app-db.default.svc.cluster.local",
    port     = "5432",
    user     = "topas",
    password = "topas",
    dbname   = "testdb"
})
print("  ✓ Database connected")

-- 2. Seed items
print("[2/5] Seeding items via db.seed()...")
db.seed({
    table = "items",
    rows = {
        { name = "widget", price = 9.99 },
        { name = "gadget", price = 19.99 }
    }
})
print("  ✓ Items seeded")

-- 3. Verify data exists in DB
print("[3/5] Verifying items via db.expect()...")
db.expect({ table = "items", where = { name = "widget" } })
db.expect({ table = "items", where = { name = "gadget" } })
print("  ✓ Items verified in database")

-- 4. Verify via HTTP API (app reads from same DB)
print("[4/5] Verifying items via HTTP GET /api/items...")
http.expect({
    url    = "http://mock-app-echo:8080/api/items",
    method = "GET",
    expect = {
        status = 200
    }
})
print("  ✓ Items endpoint returns 200")

-- 5. Verify filtered query
print("[5/5] Verifying filtered query via HTTP GET /api/items?name=widget...")
http.expect({
    url    = "http://mock-app-echo:8080/api/items?name=widget",
    method = "GET",
    expect = {
        status = 200
    }
})
print("  ✓ Filtered query returns 200")

print("=== ALL DB TESTS PASSED ===")

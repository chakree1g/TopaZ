package db

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/chakradharkondapalli/topas/pkg/lua/util"
	_ "github.com/lib/pq"
	lua "github.com/yuin/gopher-lua"
)

type Module struct {
	DB *sql.DB
}

func New() *Module {
	return &Module{}
}

func (m *Module) Loader(L *lua.LState) int {
	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"connect": m.Connect,
		"seed":    m.Seed,
		"expect":  m.Expect,
	})
	L.Push(mod)
	return 1
}

func (m *Module) Connect(L *lua.LState) int {
	// config = { type="postgres", host=..., ... } OR uri string
	arg := L.CheckAny(1)
	var dsn string
	driver := "postgres"

	if arg.Type() == lua.LTString {
		dsn = arg.String()
	} else if arg.Type() == lua.LTTable {
		// Construct DSN from table
		t := arg.(*lua.LTable)
		user := t.RawGetString("user").String()
		pass := t.RawGetString("password").String()
		host := t.RawGetString("host").String()
		port := t.RawGetString("port").String()
		dbname := t.RawGetString("dbname").String()
		if port == "" || port == "nil" {
			port = "5432"
		}
		dsn = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, dbname)
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		L.RaiseError("failed to open db: %v", err)
		return 0
	}
	if err := db.Ping(); err != nil {
		L.RaiseError("failed to connect to db: %v", err)
		return 0
	}
	m.DB = db
	return 0
}

func (m *Module) Seed(L *lua.LState) int {
	// seed({ table="users", rows={{name="CHECKOUT"}, ...} })
	arg := L.CheckTable(1)
	table := arg.RawGetString("table").String()
	rows := arg.RawGetString("rows")

	if rows.Type() != lua.LTTable {
		L.RaiseError("rows must be a table")
		return 0
	}

	rowsTable := rows.(*lua.LTable)
	rowsTable.ForEach(func(_, row lua.LValue) {
		if row.Type() != lua.LTTable {
			return
		}
		r := row.(*lua.LTable)
		cols := []string{}
		vals := []interface{}{}
		placeholders := []string{}
		i := 1
		r.ForEach(func(k, v lua.LValue) {
			cols = append(cols, k.String())
			vals = append(vals, toGoValue(v))
			placeholders = append(placeholders, fmt.Sprintf("$%d", i))
			i++
		})

		query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(cols, ", "), strings.Join(placeholders, ", "))
		if _, err := m.DB.Exec(query, vals...); err != nil {
			L.RaiseError("seed failed: %v", err)
		}
	})
	return 0
}

func (m *Module) Expect(L *lua.LState) int {
	// expect({ table="users", where={name="alice"}, json={role="admin"} })
	arg := L.CheckTable(1)
	table := arg.RawGetString("table").String()
	where := arg.RawGetString("where")

	query := fmt.Sprintf("SELECT * FROM %s", table)
	args := []interface{}{}

	if where.Type() == lua.LTTable {
		conds := []string{}
		i := 1
		where.(*lua.LTable).ForEach(func(k, v lua.LValue) {
			conds = append(conds, fmt.Sprintf("%s = $%d", k.String(), i))
			args = append(args, toGoValue(v))
			i++
		})
		if len(conds) > 0 {
			query += " WHERE " + strings.Join(conds, " AND ")
		}
	}

	rows, err := m.DB.Query(query, args...)
	if err != nil {
		L.RaiseError("query failed: %v", err)
		return 0
	}
	defer rows.Close()

	if !rows.Next() {
		L.RaiseError("assertion failed: no rows found")
		return 0
	}

	// TODO: Verify JSON content if provided
	return 0
}

func toGoValue(v lua.LValue) interface{} {
	return util.ToGoValue(v)
}

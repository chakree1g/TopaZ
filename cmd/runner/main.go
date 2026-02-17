package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/chakradharkondapalli/topas/pkg/k8s"
	ldb "github.com/chakradharkondapalli/topas/pkg/lua/db"
	lhttp "github.com/chakradharkondapalli/topas/pkg/lua/http"
	lnet "github.com/chakradharkondapalli/topas/pkg/lua/net" // Added net module import
	lpm "github.com/chakradharkondapalli/topas/pkg/lua/postman"
	lsut "github.com/chakradharkondapalli/topas/pkg/lua/sut"
	lua "github.com/yuin/gopher-lua"
)

func main() {
	scriptPath := flag.String("script", "", "Path to the Lua script")
	appName := flag.String("app", "", "Name of the App resource")
	namespace := flag.String("namespace", "default", "Namespace of the App")
	flag.Parse()

	if *scriptPath == "" || *appName == "" {
		fmt.Println("Usage: runner --script <path> --app <name> [--namespace <ns>]")
		os.Exit(1)
	}

	// 1. Initialize K8s Client
	k8sClient, err := k8s.NewClient()
	if err != nil {
		fmt.Printf("Failed to create k8s client: %v\n", err)
		os.Exit(1)
	}

	// 2. Initialize Lua State
	L := lua.NewState()
	defer L.Close()
	L.OpenLibs()

	// 3. Register Modules
	sutMod := lsut.New(k8sClient, *appName, *namespace)
	L.PreloadModule("sut", sutMod.Loader)

	httpMod := lhttp.New()
	L.PreloadModule("http", httpMod.Loader)

	dbMod := ldb.New()
	L.PreloadModule("db", dbMod.Loader)

	netMod := lnet.New() // Unified Network Client
	L.PreloadModule("net", netMod.Loader)

	pmMod := lpm.New()
	L.PreloadModule("postman", pmMod.Loader)

	// 4. Execute Script
	fmt.Printf("Executing script: %s for App: %s/%s\n", *scriptPath, *namespace, *appName)
	if err := L.DoFile(*scriptPath); err != nil {
		fmt.Printf("Error executing script: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Script execution finished successfully")
}

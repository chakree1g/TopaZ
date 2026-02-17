package sut

import (
	"context"
	"time"

	lua "github.com/yuin/gopher-lua"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/chakradharkondapalli/topas/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
)

type Module struct {
	Client    client.Client
	AppName   string
	Namespace string
}

func New(c client.Client, appName, namespace string) *Module {
	return &Module{Client: c, AppName: appName, Namespace: namespace}
}

func (m *Module) Loader(L *lua.LState) int {
	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"apply": m.Apply,
		"wait":  m.Wait,
	})
	L.Push(mod)
	return 1
}

func (m *Module) Apply(L *lua.LState) int {
	serviceName := L.CheckString(1)
	specTable := L.CheckTable(2)

	ctx := context.Background()
	app := &appv1alpha1.App{}
	err := m.Client.Get(ctx, types.NamespacedName{Name: m.AppName, Namespace: m.Namespace}, app)
	if err != nil {
		L.RaiseError("failed to get app: %v", err)
		return 0
	}

	// Update the service spec in the App CR
	updated := false
	for i, svc := range app.Spec.Services {
		if svc.Name == serviceName {
			// Parse Lua table into ServiceSpec fields
			specTable.ForEach(func(k, v lua.LValue) {
				key := k.String()
				switch key {
				case "image":
					app.Spec.Services[i].Image = v.String()
				case "version":
					app.Spec.Services[i].Version = v.String()
				case "replicas":
					val := int32(v.(lua.LNumber))
					app.Spec.Services[i].Replicas = &val
				}
			})
			updated = true
			break
		}
	}

	if !updated {
		L.RaiseError("service not found: %s", serviceName)
		return 0
	}

	if err := m.Client.Update(ctx, app); err != nil {
		L.RaiseError("failed to update app: %v", err)
		return 0
	}

	return 0
}

func (m *Module) Wait(L *lua.LState) int {
	serviceName := L.CheckString(1)
	// timeout := L.OptString(2, "60s") // TODO: Implement timeout parsing

	ctx := context.Background()
	deploymentName := m.AppName + "-" + serviceName

	// Poll for readiness
	for {
		dep := &appsv1.Deployment{}
		err := m.Client.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: m.Namespace}, dep)
		if err == nil {
			if dep.Status.ReadyReplicas == *dep.Spec.Replicas {
				return 0 // Ready
			}
		}
		time.Sleep(1 * time.Second)
		// Check for timeout
	}
}

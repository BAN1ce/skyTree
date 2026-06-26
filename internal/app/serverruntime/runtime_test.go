package serverruntime

import (
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/internal/app/clusterruntime"
	"github.com/BAN1ce/skyTree/internal/app/consoleruntime"
	"github.com/BAN1ce/skyTree/config"
)

func TestNewConsoleControlClientCreatesK8sProvider(t *testing.T) {
	client := newConsoleControlClient(
		config.AppConfig{
			Console: config.Console{
				Control: config.ConsoleControl{
					Enabled:  true,
					Provider: "k8s",
					K8s: config.ConsoleK8sControl{
						Namespace:      "skytree-local",
						LabelSelector:  "app=skytree",
						RequestTimeout: time.Second,
						JoinTimeout:    time.Second,
					},
				},
			},
		},
		&clusterruntime.Runtime{},
	)
	if _, ok := client.(*consoleruntime.K8sControlClient); !ok {
		t.Fatalf("client = %T, want *K8sControlClient", client)
	}
}

// +build linux

package libcontainer

import (
	"fmt"
	"os"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

type mockCgroupManager struct {
	pids    []int
	allPids []int
	stats   *cgroups.Stats
	paths   map[string]string
}

func (m *mockCgroupManager) GetPids() ([]int, error) {
	return m.pids, nil
}

func (m *mockCgroupManager) GetAllPids() ([]int, error) {
	return m.allPids, nil
}

func (m *mockCgroupManager) GetStats() (*cgroups.Stats, error) {
	return m.stats, nil
}

func (m *mockCgroupManager) Apply(pid int) error {
	return nil
}

func (m *mockCgroupManager) Set(container *configs.Config) error {
	return nil
}

func (m *mockCgroupManager) Destroy() error {
	return nil
}

func (m *mockCgroupManager) GetPaths() map[string]string {
	return m.paths
}

func (m *mockCgroupManager) Freeze(state configs.FreezerState) error {
	return nil
}

type mockProcess struct {
	_pid    int
	started string
}

func (m *mockProcess) terminate() error {
	return nil
}

func (m *mockProcess) pid() int {
	return m._pid
}

func (m *mockProcess) startTime() (string, error) {
	return m.started, nil
}

func (m *mockProcess) start() error {
	return nil
}

func (m *mockProcess) wait() (*os.ProcessState, error) {
	return nil, nil
}

func (m *mockProcess) signal(_ os.Signal) error {
	return nil
}

func (m *mockProcess) externalDescriptors() []string {
	return []string{}
}

func (m *mockProcess) setExternalDescriptors(newFds []string) {
}

func TestGetContainerPids(t *testing.T) {
	container := &linuxContainer{
		id:            "myid",
		config:        &configs.Config{},
		cgroupManager: &mockCgroupManager{allPids: []int{1, 2, 3}},
	}
	pids, err := container.Processes()
	if err != nil {
		t.Fatal(err)
	}
	for i, expected := range []int{1, 2, 3} {
		if pids[i] != expected {
			t.Fatalf("expected pid %d but received %d", expected, pids[i])
		}
	}
}

func TestGetContainerStats(t *testing.T) {
	container := &linuxContainer{
		id:     "myid",
		config: &configs.Config{},
		cgroupManager: &mockCgroupManager{
			pids: []int{1, 2, 3},
			stats: &cgroups.Stats{
				MemoryStats: cgroups.MemoryStats{
					Usage: cgroups.MemoryData{
						Usage: 1024,
					},
				},
			},
		},
	}
	stats, err := container.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.CgroupStats == nil {
		t.Fatal("cgroup stats are nil")
	}
	if stats.CgroupStats.MemoryStats.Usage.Usage != 1024 {
		t.Fatalf("expected memory usage 1024 but recevied %d", stats.CgroupStats.MemoryStats.Usage.Usage)
	}
}

func TestGetContainerState(t *testing.T) {
	var (
		pid                 = os.Getpid()
		expectedMemoryPath  = "/sys/fs/cgroup/memory/myid"
		expectedNetworkPath = "/networks/fd"
	)
	container := &linuxContainer{
		id: "myid",
		config: &configs.Config{
			Namespaces: []configs.Namespace{
				{Type: configs.NEWPID},
				{Type: configs.NEWNS},
				{Type: configs.NEWNET, Path: expectedNetworkPath},
				{Type: configs.NEWUTS},
				// emulate host for IPC
				//{Type: configs.NEWIPC},
			},
		},
		initProcess: &mockProcess{
			_pid:    pid,
			started: "010",
		},
		cgroupManager: &mockCgroupManager{
			pids: []int{1, 2, 3},
			stats: &cgroups.Stats{
				MemoryStats: cgroups.MemoryStats{
					Usage: cgroups.MemoryData{
						Usage: 1024,
					},
				},
			},
			paths: map[string]string{
				"memory": expectedMemoryPath,
			},
		},
	}
	container.state = &createdState{c: container}
	state, err := container.State()
	if err != nil {
		t.Fatal(err)
	}
	if state.InitProcessPid != pid {
		t.Fatalf("expected pid %d but received %d", pid, state.InitProcessPid)
	}
	if state.InitProcessStartTime != "010" {
		t.Fatalf("expected process start time 010 but received %s", state.InitProcessStartTime)
	}
	paths := state.CgroupPaths
	if paths == nil {
		t.Fatal("cgroup paths should not be nil")
	}
	if memPath := paths["memory"]; memPath != expectedMemoryPath {
		t.Fatalf("expected memory path %q but received %q", expectedMemoryPath, memPath)
	}
	for _, ns := range container.config.Namespaces {
		path := state.NamespacePaths[ns.Type]
		if path == "" {
			t.Fatalf("expected non nil namespace path for %s", ns.Type)
		}
		if ns.Type == configs.NEWNET {
			if path != expectedNetworkPath {
				t.Fatalf("expected path %q but received %q", expectedNetworkPath, path)
			}
		} else {
			file := ""
			switch ns.Type {
			case configs.NEWNET:
				file = "net"
			case configs.NEWNS:
				file = "mnt"
			case configs.NEWPID:
				file = "pid"
			case configs.NEWIPC:
				file = "ipc"
			case configs.NEWUSER:
				file = "user"
			case configs.NEWUTS:
				file = "uts"
			}
			expected := fmt.Sprintf("/proc/%d/ns/%s", pid, file)
			if expected != path {
				t.Fatalf("expected path %q but received %q", expected, path)
			}
		}
	}
}

func TestParseState(t *testing.T) {
	data := map[string]int{
		"4902 (gunicorn: maste) S 4885 4902 4902 0 -1 4194560 29683 29929 61 83 78 16 96 17 20 0 1 0 9126532 52965376 1903 18446744073709551615 4194304 7461796 140733928751520 140733928698072 139816984959091 0 0 16781312 137447943 1 0 0 17 3 0 0 9 0 0 9559488 10071156 33050624 140733928758775 140733928758945 140733928758945 140733928759264 0": 'S',
		"9534 (cat) R 9323 9534 9323 34828 9534 4194304 95 0 0 0 0 0 0 0 20 0 1 0 9214966 7626752 168 18446744073709551615 4194304 4240332 140732237651568 140732237650920 140570710391216 0 0 0 0 0 0 0 17 1 0 0 0 0 0 6340112 6341364 21553152 140732237653865 140732237653885 140732237653885 140732237656047 0":                                      'R',

		"24767 (irq/44-mei_me) S 2 0 0 0 -1 2129984 0 0 0 0 0 0 0 0 -51 0 1 0 8722075 0 0 18446744073709551615 0 0 0 0 0 0 0 2147483647 0 0 0 0 17 1 50 1 0 0 0 0 0 0 0 0 0 0 0": 'S',
	}
	for line, expected := range data {
		state, err := parseState(line)
		if err != nil {
			t.Fatal(err)
		}
		if state != expected {
			t.Fatalf("expected state %q but received %q", expected, state)
		}
	}
}

package main

import (
	"fmt"
	"os"
	"strings"
)

// Read-only cgroup v2 counters. No shell, privileges or Docker socket in the image.
func memoryStats() error {
	read := func(name string) (string, error) {
		b, e := os.ReadFile("/sys/fs/cgroup/" + name)
		return strings.TrimSpace(string(b)), e
	}
	current, e := read("memory.current")
	if e != nil {
		return fmt.Errorf("cgroup v2 memory counters unavailable")
	}
	peak, e := read("memory.peak")
	if e != nil {
		peak = "unavailable"
	}
	max, e := read("memory.max")
	if e != nil {
		return e
	}
	events, e := read("memory.events")
	if e != nil {
		return e
	}
	counters := map[string]string{}
	for _, line := range strings.Split(events, "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 {
			counters[parts[0]] = parts[1]
		}
	}
	fmt.Printf("%s,%s,%s,%s,%s\n", current, peak, max, counters["oom"], counters["oom_kill"])
	return nil
}

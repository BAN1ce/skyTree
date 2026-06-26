package scheduler

import (
	"strconv"
	"testing"
	"time"
)

type workloadProfile struct {
	name             string
	clients          int
	retryRate        float64
	scheduledTasks   int
	dueTasksPerTick  int
	retryIngressPerS int
}

var benchmarkWorkloadProfiles = []workloadProfile{
	{
		name:             "small",
		clients:          1000,
		retryRate:        0.01,
		scheduledTasks:   100,
		dueTasksPerTick:  10,
		retryIngressPerS: 100,
	},
	{
		name:             "medium",
		clients:          10000,
		retryRate:        0.02,
		scheduledTasks:   2000,
		dueTasksPerTick:  100,
		retryIngressPerS: 1000,
	},
	{
		name:             "large",
		clients:          50000,
		retryRate:        0.05,
		scheduledTasks:   10000,
		dueTasksPerTick:  500,
		retryIngressPerS: 5000,
	},
	{
		name:             "xlarge",
		clients:          100000,
		retryRate:        0.10,
		scheduledTasks:   30000,
		dueTasksPerTick:  3000,
		retryIngressPerS: 10000,
	},
}

func BenchmarkPassiveSchedulerWorkloadProfiles(b *testing.B) {
	now := time.Now()

	for _, profile := range benchmarkWorkloadProfiles {
		profile := profile

		b.Run(profile.name+"/create", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				tw := NewPassiveScheduler(time.Second)
				for j := 0; j < profile.scheduledTasks; j++ {
					tw.Add(&testTask{
						key:        profile.name + "-task-" + strconv.Itoa(i) + "-" + strconv.Itoa(j),
						expireTime: now.Add(30 * time.Second).UnixMicro(),
					})
				}
			}
		})

		b.Run(profile.name+"/tick-no-due", func(b *testing.B) {
			tw := NewPassiveScheduler(time.Second)
			for i := 0; i < profile.scheduledTasks; i++ {
				tw.Add(&testTask{
					key:        profile.name + "-pending-" + strconv.Itoa(i),
					expireTime: now.Add(30 * time.Second).UnixMicro(),
				})
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tw.GetExpired(now.UnixMicro())
			}
		})

		b.Run(profile.name+"/tick-burst-due", func(b *testing.B) {
			current := now.UnixMicro()
			tw := NewPassiveScheduler(time.Second)
			for i := 0; i < profile.dueTasksPerTick; i++ {
				tw.Add(&testTask{
					key:        profile.name + "-due-" + strconv.Itoa(i),
					expireTime: current - int64(i+1),
				})
			}
			for i := profile.dueTasksPerTick; i < profile.scheduledTasks; i++ {
				tw.Add(&testTask{
					key:        profile.name + "-future-" + strconv.Itoa(i),
					expireTime: current + int64(time.Minute/time.Microsecond),
				})
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tw.GetExpired(current)
			}
		})
	}
}

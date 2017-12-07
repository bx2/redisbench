package main

import (
	"fmt"
	"log"
	"redisbench/config"
	"redisbench/models"
	"redisbench/tester"
	"redisbench/utils"
	"redisbench/wares"
	"time"

	"github.com/go-redis/redis"
)

func clientRun(id int, times int, size int, redisClient redis.UniversalClient) {
	defer tester.Wg.Done()
	val := utils.RandSeq(size)
	var err error
	for i := 0; i < times; i++ {
		key := fmt.Sprintf("benchmark.set.%d.%d", id, i)
		err = redisClient.Set(key, val, 0).Err()
		utils.FatalErr(err)
	}
}

func main() {
	// Parse config arguments from command-line
	config.Parse()
	if config.MultiAddr != "" {
		tester.RPCRun()
	}

	tester.Wg.Wait()
	log.Println("Go...")

	// Print test initial information
	totalTimes := int64(config.ClientNum * config.TestTimes)
	totalSize := int64(config.ClientNum * config.DataSize)
	log.Printf("# BENCHMARK (CLUSTER: %v)", config.ClusterMode)
	log.Printf("* ClientNumber:%d, TestTimes:%d, DataSize:%d", config.ClientNum, config.TestTimes, config.DataSize)
	log.Printf("* TotalTimes:%d, TotalSize:%d", totalTimes, totalSize)

	// Create a new redis client
	redisClient, err := wares.NewUniversalRedisClient()
	utils.FatalErr(err)

	// Run cerain number clients for testing
	t1 := utils.NowMilliTs()
	for i := 0; i < config.ClientNum; i++ {
		tester.Wg.Add(1)
		go clientRun(i, config.TestTimes, config.DataSize, redisClient)
	}
	tester.Wg.Wait()
	t2 := utils.NowMilliTs()

	// Calculate duration while done (can't be 0, min is 1)
	dur := t2 - t1
	order := 1
	if tester.Multi != nil {
		order = tester.Multi.Order
	}
	result := &models.NodeResult{Order: order, TotalTimes: totalTimes, TsBeg: t1, TsEnd: t2, TotalDur: dur}
	tps := int(result.TotalTimes / (result.TotalDur / 1000.0))
	log.Println("# BENCHMARK DONE")
	log.Printf("* SUM: %d, DUR: %0.3fs, TPS: %d", result.TotalDur, float64(result.TotalDur)/1000, tps)

	if tester.Multi != nil {
		if !tester.Multi.IsMaster() {
			// Notice master to settle
			tester.Multi.NoticeMasterSettle(result)
			log.Println("see summary info on node 1")
		} else {
			tester.Wg.Add(1) // Wait all others nodes settling call
			tester.Multi.NodeSettle(result)

			tester.Wg.Wait()
			time.Sleep(time.Second)
			// Summary all nodes result include self
			summary := tester.Multi.Summary()

			// Print testing result
			log.Println("# SUMMARY")
			log.Printf("* SUM: %d, DUR: %.3fs, TPS: %d", summary.TotalTimes, float64(summary.TotalDur)/1000, summary.TPS)
		}
	}
}

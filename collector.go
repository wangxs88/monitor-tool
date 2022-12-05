package monitor_tool

import "time"

// Every piece of reported data flows into the collection module, which only does some simple data logging
type reportData struct {
	// Unique naming of entries
	Name string
	// Total time taken for success
	SuccessMsCount uint64
	// Maximum time taken for success
	MaxMs uint32
	// Minimum time required for success
	MinMs uint32
	// Total number of successes
	SuccessCount uint32
	// Total number of time attainment
	FastCount uint32
	// Total number of failures
	FailCount uint32
	// Failure Distribution By Status Code
	FailDistribution map[int]uint32
	// Time delay distribution
	TimeConsumingDistribution []uint32
	// Configuration of entries
	Config *EntryConfig
	// Time of this count
	Time time.Time
}

// EntryConfig More detailed configuration related to item statistics
type EntryConfig struct {
	// The maximum time taken for time attainment, default is 500ms
	FastLessThan uint32
	// The number of intervals in the time consumption distribution, the default is 10, at least 3, at most 20
	// The first interval is used to mark the number of intervals less than TimeConsumingMin
	// The last interval is used to mark the number of intervals greater than TimeConsumingMax
	// the remaining (TimeConsumingDistributionSplit - 2) intervals, in the range (MaxConsumingMin)/(Number of intervals - 2)
	TimeConsumingDistributionSplit int
	TimeConsumingDistributionMax   uint32
	TimeConsumingDistributionMin   uint32
	timeConsumingRange             uint32
}

var defaultEntryConfig = &EntryConfig{
	FastLessThan:                   500,
	TimeConsumingDistributionSplit: 10,
	TimeConsumingDistributionMax:   500,
	TimeConsumingDistributionMin:   100,
	timeConsumingRange:             (500 - 100) / (10 - 2),
}

func (c *ReportClientConfig) getEntryConfig(name string) *EntryConfig {
	if curEntryConfig, ok := c.entryConfigMap[name]; ok {
		return &curEntryConfig
	}
	return defaultEntryConfig
}

func (c *ReportClientConfig) AddEntryConfig(name string, entryConfig EntryConfig) {
	if entryConfig.FastLessThan <= 0 {
		entryConfig.FastLessThan = 500
	}
	if entryConfig.TimeConsumingDistributionSplit < 3 || entryConfig.TimeConsumingDistributionSplit > 20 {
		entryConfig.TimeConsumingDistributionSplit = 10
	}
	if entryConfig.TimeConsumingDistributionMax <= 0 {
		entryConfig.TimeConsumingDistributionMax = 500
	}
	if entryConfig.TimeConsumingDistributionMin <= 0 {
		entryConfig.TimeConsumingDistributionMin = 50
	}
	if entryConfig.TimeConsumingDistributionMax <= entryConfig.TimeConsumingDistributionMin {
		panic("The maximum elapsed time must be greater than the minimum elapsed time")
	}
	entryConfig.timeConsumingRange = (entryConfig.TimeConsumingDistributionMax - entryConfig.TimeConsumingDistributionMin) / uint32(entryConfig.TimeConsumingDistributionSplit-2)
	c.entryConfigMap[name] = entryConfig
}

// Collection
func (c *ReportClientConfig) collect() {
	// Listening to this client's uplink channel
	for t := range c.taskChannel {
		if t.taskType == SERVER {
			curReportServerData := t.data.(reportServer)
			c.serverTask(&curReportServerData)

		} else if t.taskType == CLEAR {
			curClearData := t.data.(clearData)
			c.clearTask(&curClearData)
		}
	}
}

func (c *ReportClientConfig) clearTask(curClearData *clearData) {
	curCollectData := c.collectDataMap[curClearData.Name]
	if curCollectData.SuccessCount != 0 || curCollectData.FailCount != 0 {
		collectedData := *curCollectData
		collectedData.Time = curClearData.Time
		// A copy of the data flows into the analysis
		c.statisticsChannel <- collectedData
		curCollectData.MinMs = 0
		curCollectData.MaxMs = 0
		curCollectData.FailCount = 0
		curCollectData.SuccessCount = 0
		curCollectData.SuccessMsCount = 0
		curCollectData.FastCount = 0
		curCollectData.FailDistribution = map[int]uint32{}
		curCollectData.TimeConsumingDistribution = make([]uint32, curCollectData.Config.TimeConsumingDistributionSplit)
	}
}

func (c *ReportClientConfig) serverTask(curReportServerData *reportServer) {
	if c.collectDataMap[curReportServerData.Name] == nil {
		c.collectDataMap[curReportServerData.Name] = &reportData{
			Name:             curReportServerData.Name,
			Config:           c.getEntryConfig(curReportServerData.Name),
			FailDistribution: map[int]uint32{},
		}
	}
	curCollectData := c.collectDataMap[curReportServerData.Name]
	if curCollectData.TimeConsumingDistribution == nil {
		curCollectData.TimeConsumingDistribution = make([]uint32, curCollectData.Config.TimeConsumingDistributionSplit)
	}
	var success bool
	if c.GetCodeFeature != nil {
		success, _ = c.GetCodeFeature(curReportServerData.Code)
	} else if s, ok := c.CodeFeatureMap[curReportServerData.Code]; ok {
		success = s.Success
	}
	// Hit success status code
	if success {
		curCollectData.SuccessCount++
		if curCollectData.MinMs == 0 {
			curCollectData.MinMs = curReportServerData.Ms
		} else if curReportServerData.Ms < curCollectData.MinMs {
			curCollectData.MinMs = curReportServerData.Ms
		}
		if curCollectData.MaxMs == 0 {
			curCollectData.MaxMs = curReportServerData.Ms
		} else if curReportServerData.Ms > curCollectData.MaxMs {
			curCollectData.MaxMs = curReportServerData.Ms
		}
		curCollectData.SuccessMsCount += uint64(curReportServerData.Ms)
		// Time consumed is less than the minimum of
		//the interval Classified as the first interval
		if curReportServerData.Ms < curCollectData.Config.TimeConsumingDistributionMin {
			curCollectData.TimeConsumingDistribution[0] += 1
		} else if curReportServerData.Ms >= curCollectData.Config.TimeConsumingDistributionMax {
			curCollectData.TimeConsumingDistribution[curCollectData.Config.TimeConsumingDistributionSplit-1] += 1
		} else {
			curCollectData.TimeConsumingDistribution[(curReportServerData.Ms-curCollectData.Config.TimeConsumingDistributionMin)/curCollectData.Config.timeConsumingRange+1] += 1
		}
		if curReportServerData.Ms <= curCollectData.Config.FastLessThan {
			curCollectData.FastCount++
		}
	} else {
		curCollectData.FailCount++
		curCollectData.FailDistribution[curReportServerData.Code]++
	}
}

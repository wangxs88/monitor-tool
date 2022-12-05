package monitor_tool

import (
	"strconv"
	"strings"
	"time"
)

// OutPutData Final statistical output in one time period
type OutPutData struct {
	// Data generation time
	Timestamp time.Time `json:"timestamp"`
	// Client Naming
	ClientName string `json:"clientName"`
	// Interface Naming
	InterfaceName string `json:"interfaceName"`
	// Total number of calls
	Count uint32 `json:"count"`
	// Total number of successes
	SuccessCount uint32 `json:"successCount"`
	// Success rate
	SuccessRate float64 `json:"successRate"`
	// Average time taken for success
	SuccessMsAver uint32 `json:"successMsAver"`
	// Maximum time taken for success
	MaxMs uint32 `json:"maxMs"`
	// Minimum time required for success
	MinMs uint32 `json:"minMs"`
	// Total number of time attainment
	FastCount uint32 `json:"fastCount"`
	// Time compliance rate
	FastRate float64 `json:"fastRate"`
	// Total number of failures
	FailCount uint32 `json:"failCount"`
	// Failure Distribution By Status Code
	FailDistribution map[string]uint32 `json:"failDistribution"`
	// Time delay distribution
	TimeConsumingDistribution map[string]uint32 `json:"timeConsumingDistribution"`
}

// Store some recent state for alerting, recovery and other mechanisms
type alertStatus struct {
	recentAlertOutput   []OutPutData // The last few consecutive failed data
	recentRecoverOutput []OutPutData // Several consecutive successful data since the most recent alarm
	curState            AlertType    // Whether the current state is in the detection of recovery after an alarm
}

// Periodic start-up analysis tasks
func (c *ReportClientConfig) scheduleTask() {
	// Timed statistics
	t := time.NewTicker(time.Duration(c.StatisticalCycle) * time.Millisecond)
	for curTime := range t.C {
		for _, collectData := range c.collectDataMap {
			c.taskChannel <- &taskQueue{
				taskType: CLEAR,
				data: clearData{
					Name: collectData.Name,
					Time: curTime,
				},
			}
		}
	}
}

// Analysis Statistics
func (c *ReportClientConfig) statistics() {
	// Statistical analysis in terms of specific entries
	for collectedData := range c.statisticsChannel {
		// Statistics of general indicators
		outputData := OutPutData{}
		outputData.ClientName = c.Name
		outputData.InterfaceName = collectedData.Name
		outputData.Count = collectedData.FailCount + collectedData.SuccessCount
		outputData.SuccessRate = float64(collectedData.SuccessCount) / float64(outputData.Count)
		outputData.FastRate = float64(collectedData.FastCount) / float64(outputData.Count)
		outputData.FastCount = collectedData.FastCount
		outputData.SuccessMsAver = uint32(float64(collectedData.SuccessMsCount) / float64(outputData.Count))
		outputData.SuccessCount = collectedData.SuccessCount
		outputData.FailCount = collectedData.FailCount
		outputData.MaxMs = collectedData.MaxMs
		outputData.MinMs = collectedData.MinMs
		outputData.Timestamp = collectedData.Time.UTC()
		outputData.TimeConsumingDistribution = map[string]uint32{}
		outputData.FailDistribution = map[string]uint32{}

		// Time delay distribution statistics
		scope := (collectedData.Config.TimeConsumingDistributionMax - collectedData.Config.TimeConsumingDistributionMin) / uint32(collectedData.Config.TimeConsumingDistributionSplit-2)
		// Calculate the first interval
		outputData.TimeConsumingDistribution["<"+strconv.FormatUint(uint64(collectedData.Config.TimeConsumingDistributionMin), 10)] = collectedData.TimeConsumingDistribution[0]
		// Calculate the last interval
		outputData.TimeConsumingDistribution[">"+strconv.FormatUint(uint64(collectedData.Config.TimeConsumingDistributionMax), 10)] = collectedData.TimeConsumingDistribution[collectedData.Config.TimeConsumingDistributionSplit-1]
		// Calculate the remaining interval
		for i := 1; i < collectedData.Config.TimeConsumingDistributionSplit-1; i++ {
			start := int(collectedData.Config.TimeConsumingDistributionMin + uint32(i-1)*scope)
			var end int
			if i == collectedData.Config.TimeConsumingDistributionSplit-1 {
				end = int(collectedData.Config.TimeConsumingDistributionMax)
			} else {
				end = int(collectedData.Config.TimeConsumingDistributionMin + uint32(i)*scope)
			}
			outputData.TimeConsumingDistribution[strconv.Itoa(start)+"~"+strconv.Itoa(end)] = collectedData.TimeConsumingDistribution[i]
		}

		// Failure distribution statistics
		for status, count := range collectedData.FailDistribution {
			var name string
			if c.GetCodeFeature != nil {
				_, name = c.GetCodeFeature(status)
			} else if s, ok := c.CodeFeatureMap[status]; ok && s.Name != "" {
				name = s.Name
			}
			if name != "" {
				outputData.FailDistribution[name] = count
			} else {
				outputData.FailDistribution[strings.Replace(c.DefaultFailDistributionFormat, "%code", strconv.Itoa(status), 1)] = count
			}
		}

		// Alarm analysis: Since alarm analysis has the possibility of calling customized alarm functions,
		//performance cannot be predicted,
		//so new goroutine is enabled to perform to avoid unpredictable risks
		go c.alertAnalyze(collectedData.Name, outputData)

		// Output final statistics
		if c.OutputCaller != nil {
			// Similarly, any external custom function
			//calls should be executed with a new goroutine enabled
			go c.OutputCaller(&outputData)
		}
		defaultOutputCaller(&outputData)
	}
}

// Alarm-related analysis
func (c *ReportClientConfig) alertAnalyze(entryName string, outputData OutPutData) {
	// Latency compliance alarm and recovery analysis
	if _, ok := c.recentFastRateStatus[entryName]; !ok {
		c.recentFastRateStatus[entryName] = &alertStatus{
			recentAlertOutput: make([]OutPutData, 0),
		}
	}
	curFastRateStatus := c.recentFastRateStatus[entryName]
	if _, ok := c.recentSuccessRateStatus[entryName]; !ok {
		c.recentSuccessRateStatus[entryName] = &alertStatus{
			recentAlertOutput: make([]OutPutData, 0),
		}
	}
	curSuccessRateStatus := c.recentSuccessRateStatus[entryName]
	// Latency non-compliance alerts only trigger statistics when there are successful requests
	if outputData.SuccessCount > 0 && outputData.FastRate < c.FastRate {
		// Each failure will reset the recovery count
		if len(curFastRateStatus.recentRecoverOutput) > 0 {
			curFastRateStatus.recentRecoverOutput = curFastRateStatus.recentRecoverOutput[:0]
		}
		curFastRateStatus.recentAlertOutput = append(curFastRateStatus.recentAlertOutput, outputData)
		if curFastRateStatus.curState == NONE && len(curFastRateStatus.recentAlertOutput) >= c.AlertForBadFastRateReachedTimes {
			// Mark the status of the current alarm
			curFastRateStatus.curState = SLOW
			if c.AlertCaller != nil {
				c.AlertCaller(c.Name, entryName, SLOW, curFastRateStatus.recentAlertOutput)
			} else {
				defaultAlert(c.Name, entryName, SLOW, curFastRateStatus.recentAlertOutput)
			}
			curFastRateStatus.recentAlertOutput = curFastRateStatus.recentAlertOutput[:0]
		}
	} else {
		// As long as a success to clear the original unhealthy records, the performance is
		//slightly better than judging whether the length is greater than 0 before clearing
		curFastRateStatus.recentAlertOutput = curFastRateStatus.recentAlertOutput[:0]
		//  When in alarm status, the number of recoveries is accumulated for each success
		if curFastRateStatus.curState == SLOW {
			curFastRateStatus.recentRecoverOutput = append(curFastRateStatus.recentRecoverOutput, outputData)
			if len(curFastRateStatus.recentRecoverOutput) >= c.AlertForGreatFastRateReachedTimes {
				// Trigger recovery notification
				if c.RecoverCaller != nil {
					c.RecoverCaller(c.Name, entryName, SLOW, curFastRateStatus.recentRecoverOutput)
				} else {
					defaultRecover(c.Name, entryName, SLOW, curFastRateStatus.recentRecoverOutput)
				}
				// Reset flag
				curFastRateStatus.curState = NONE
				curFastRateStatus.recentAlertOutput = curFastRateStatus.recentAlertOutput[:0]
			}
		}
	}

	// Access success rate alert and recovery analysis
	if outputData.SuccessRate < c.SuccessRate {
		// Each failure will reset the recovery count
		if len(curSuccessRateStatus.recentRecoverOutput) > 0 {
			curSuccessRateStatus.recentRecoverOutput = curSuccessRateStatus.recentRecoverOutput[:0]
		}
		curSuccessRateStatus.recentAlertOutput = append(curSuccessRateStatus.recentAlertOutput, outputData)
		if curSuccessRateStatus.curState == NONE && len(curSuccessRateStatus.recentAlertOutput) >= c.AlertForBadSuccessRateReachedTimes {
			// Mark the status of the current alarm
			curSuccessRateStatus.curState = FAIL
			// Trigger the alarm of continuous time consumption not meeting the standard
			if c.AlertCaller != nil {
				c.AlertCaller(c.Name, entryName, FAIL, curSuccessRateStatus.recentAlertOutput)
			} else {
				defaultAlert(c.Name, entryName, FAIL, curSuccessRateStatus.recentAlertOutput)
			}
			curSuccessRateStatus.recentAlertOutput = curSuccessRateStatus.recentAlertOutput[:0]
		}
	} else {
		// Just one success to clear the original unhealthy record
		curSuccessRateStatus.recentAlertOutput = curSuccessRateStatus.recentAlertOutput[:0]
		//  When in alarm status, the number
		// of recoveries is accumulated for each success
		if curSuccessRateStatus.curState == FAIL {
			curSuccessRateStatus.recentRecoverOutput = append(curSuccessRateStatus.recentRecoverOutput, outputData)
			if len(curSuccessRateStatus.recentRecoverOutput) >= c.AlertForGreatSuccessRateReachedTimes {
				// Trigger recovery notification
				if c.RecoverCaller != nil {
					c.RecoverCaller(c.Name, entryName, FAIL, curSuccessRateStatus.recentRecoverOutput)
				} else {
					defaultRecover(c.Name, entryName, FAIL, curSuccessRateStatus.recentRecoverOutput)
				}
				// Reset flag
				curSuccessRateStatus.curState = NONE
				curSuccessRateStatus.recentAlertOutput = curSuccessRateStatus.recentAlertOutput[:0]
			}
		}
	}
}

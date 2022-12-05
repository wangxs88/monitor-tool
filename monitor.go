package monitor_tool

import (
	"encoding/json"
	"os"
)

type (
	// AlertType Enumeration of monitoring alarm types
	AlertType uint8
	// TaskType Queue task type enumeration
	TaskType uint8
)

const (
	// NONE Health Status
	NONE AlertType = iota
	// FAIL Access Success Rate Alerts
	FAIL
	SLOW
)

const (
	_ TaskType = iota
	SERVER
	CLEAR
)

type ReportClient interface {
	Report(name string, ms uint32, code int)
	// AddEntryConfig Add custom entry configuration, including data such as time consumption
	//criteria and latency distribution for the entry
	AddEntryConfig(name string, entryConfig EntryConfig)
}

// ReportClientConfig Global configuration of the client, a client may report several interfaces
type ReportClientConfig struct {
	Name                                 string
	DefaultFastTime                      uint32
	StatisticalCycle                     int
	AlertForBadSuccessRateReachedTimes   int
	AlertForBadFastRateReachedTimes      int
	AlertForGreatSuccessRateReachedTimes int
	AlertForGreatFastRateReachedTimes    int
	SuccessRate                          float64
	FastRate                             float64
	ChannelCacheCount                    int
	CodeFeatureMap                       map[int]CodeFeature
	GetCodeFeature                       func(code int) (success bool, name string)
	DefaultFailDistributionFormat        string
	OutputCaller                         func(o *OutPutData)
	AlertCaller                          func(clientName string, interfaceName string, alertType AlertType, recentOutputData []OutPutData)
	// Recovery notification handling customization, same as AlertCaller
	RecoverCaller func(clientName string, interfaceName string, alertType AlertType, recentOutputData []OutPutData)

	// Customize the url or name the attribute about the time-consuming reach, distribution interval,
	//etc. To maintain internal key consistency, you need to call the method to set this property
	entryConfigMap          map[string]EntryConfig
	recentSuccessRateStatus map[string]*alertStatus
	recentFastRateStatus    map[string]*alertStatus
	taskChannel             chan *taskQueue
	collectDataMap          map[string]*reportData
	statisticsChannel       chan reportData
}

type CodeFeature struct {
	Success bool
	Name    string
}

// Register You have to register first to get a
//unique client before you can use the upload
func Register(c ReportClientConfig) ReportClient {
	if c.Name == "" {
		panic("A name must be registered for this reporting type")
	}
	// Maximum of 5 minutes allowed for a statistical cycle
	if c.StatisticalCycle <= 0 || c.StatisticalCycle > 300000 {
		c.StatisticalCycle = 60000
	}
	if c.AlertForBadFastRateReachedTimes < 3 {
		c.AlertForBadFastRateReachedTimes = 3
	}
	if c.AlertForGreatFastRateReachedTimes < 3 {
		c.AlertForGreatFastRateReachedTimes = 3
	}
	if c.AlertForBadSuccessRateReachedTimes < 3 {
		c.AlertForBadSuccessRateReachedTimes = 3
	}
	if c.AlertForGreatSuccessRateReachedTimes < 3 {
		c.AlertForGreatSuccessRateReachedTimes = 3
	}
	if c.SuccessRate == 0 {
		c.SuccessRate = 0.95
	}
	if c.FastRate == 0 {
		c.FastRate = 0.8
	}
	if c.ChannelCacheCount <= 0 {
		c.ChannelCacheCount = 100
	}
	if c.DefaultFastTime > 0 {
		defaultEntryConfig.FastLessThan = c.DefaultFastTime
	}
	if c.DefaultFailDistributionFormat == "" {
		c.DefaultFailDistributionFormat = "code[%code]"
	}
	c.entryConfigMap = map[string]EntryConfig{}
	c.recentFastRateStatus = map[string]*alertStatus{}
	c.recentSuccessRateStatus = map[string]*alertStatus{}
	// If no custom code feature recognition function is
	//specified and the status code mapping is empty, then the default mechanism is enabled
	if c.GetCodeFeature == nil && c.CodeFeatureMap == nil {
		c.CodeFeatureMap = map[int]CodeFeature{
			200: {
				Success: true,
			},
		}
	}
	client := &c
	client.taskChannel = make(chan *taskQueue, c.ChannelCacheCount)
	client.statisticsChannel = make(chan reportData, c.ChannelCacheCount)
	client.collectDataMap = map[string]*reportData{}
	go client.collect()
	go client.scheduleTask()
	go client.statistics()
	return client
}

func defaultOutputCaller(o *OutPutData) {
	b, err := json.Marshal(*o)
	if err != nil {
		os.Stderr.WriteString(err.Error())
	} else {
		os.Stdout.Write(b)
		os.Stdout.WriteString("\n")
	}
}

package monitor_tool

import "time"

// Data carried by the Quality of Service Statistics task (currently not considering reporting time)
type reportServer struct {
	// Naming, which should ensure uniqueness.
	// It can be set as a combination of access address and request method when used in interface reporting.
	// Some interfaces may carry path parameters or request parameters, if not handled,
	//the monitoring results will not be as expected,
	//it is recommended to remove the request parameters and format the path parameters in advance.
	Name string
	Ms   uint32
	Code int
}

type clearData struct {
	Name string
	Time time.Time
}

// Task queues, aggregating reads and writes
//of shared variables into task queues to avoid locks
type taskQueue struct {
	taskType TaskType
	data     interface{}
}

// Report The name of the report can be named for the interface, or can be passed directly into the url
// Usually url or name is used as the report categorization criteria, but if the report is url,
//it is not directly categorized in the restful style interface specification, so the request
//method becomes especially important.
// Using a channel as the reporting method instead of considering direct synchronous reporting
//analysis is to keep the cost of reporting as low as possible, to do only what needs to be done,
//and to return immediately
// Because in concurrent scenarios, analysis requires concurrent locks, and concurrent locks are blocking
//(locks are occupied), so the analysis process can be light enough to cause reporting to affect the
//progress of the main process
// The data for the report can come from anywhere, including interface reporting, service inlining, etc.
func (c *ReportClientConfig) Report(name string, ms uint32, code int) {
	if c.taskChannel == nil {
		panic("Please first register this report type")
	}
	c.taskChannel <- &taskQueue{
		taskType: SERVER,
		data: reportServer{
			Code: code,
			Ms:   ms,
			Name: name,
		},
	}
}

package monitor_tool

import (
	"bytes"
	"os"
	"strconv"
)

// Default Alarm Handling
func defaultAlert(clientName string, interfaceName string, alertType AlertType, recentOutputData []OutPutData) {
	alertTypeString := "Unknown"
	if alertType == SLOW {
		alertTypeString = "Time delay compliance rate"
	} else if alertType == FAIL {
		alertTypeString = "Access Success Rate"
	}
	var alertString bytes.Buffer
	alertString.WriteString("\n Alerts：\n   Client reporting type：" + clientName + "\n   Interface：" + interfaceName + "\n   Alarm Type：" + alertTypeString + "\n   Recent" + strconv.Itoa(len(recentOutputData)) + "Status：")
	for i, r := range recentOutputData {
		var rate float64
		if alertType == SLOW {
			rate = r.FastRate
		} else if alertType == FAIL {
			rate = r.SuccessRate
		}
		alertString.WriteString("\n     " + strconv.Itoa(i+1) + ". " + "Call" + strconv.FormatUint(uint64(r.Count), 10) + "times，" + alertTypeString + "for" + strconv.FormatFloat(float64(rate*100), 'f', 2, 64) + "%")
	}
	os.Stderr.WriteString(alertString.String() + "\n")
}

// Default recovery notification handling
func defaultRecover(clientName string, interfaceName string, alertType AlertType, recentOutputData []OutPutData) {
	alertTypeString := "Unknown"
	if alertType == SLOW {
		alertTypeString = "Time delay compliance rate"
	} else if alertType == FAIL {
		alertTypeString = "Access Success Rate"
	}
	var alertString bytes.Buffer
	alertString.WriteString("\n Recovery Notice：\n   Client reporting type：" + clientName + "\n   Interface：" + interfaceName + "\n   Recovery Type：" + alertTypeString + "\n   Recent" + strconv.Itoa(len(recentOutputData)) + "Status：")
	for i, r := range recentOutputData {
		var rate float64
		if alertType == SLOW {
			rate = r.FastRate
		} else if alertType == FAIL {
			rate = r.SuccessRate
		}
		alertString.WriteString("\n     " + strconv.Itoa(i+1) + ". " + "Call" + strconv.FormatUint(uint64(r.Count), 10) + "times，" + alertTypeString + "for" + strconv.FormatFloat(float64(rate*100), 'f', 2, 64) + "%")
	}
	os.Stderr.WriteString(alertString.String() + "\n")
}

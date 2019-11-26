package parser

import (
	"strconv"

	"github.com/influxdata/go-syslog/rfc5424"
)

type MetricsFilter struct {
}

func (f MetricsFilter) Filter(pMes *rfc5424.SyslogMessage) map[string]interface{} {
	data := make(map[string]interface{})
	data["@source"] = map[string]interface{}{
		"type":    "metrics",
		"details": "",
	}
	data["@type"] = "Metrics"
	structData := *pMes.StructuredData()
	if gauge, ok := structData[gaugeStructuredDataID]; ok {
		data["@metric"] = f.filterGauge(gauge)
		delete(structData, gaugeStructuredDataID)
	}
	if counter, ok := structData[counterStructuredDataID]; ok {
		data["@metric"] = f.filterCounter(counter)
		delete(structData, counterStructuredDataID)
	}
	if timer, ok := structData[timerStructuredDataID]; ok {
		data["@metric"] = f.filterTimer(timer)
		delete(structData, timerStructuredDataID)
	}
	structDataPtr := pMes.StructuredData()
	*structDataPtr = structData
	return data
}

func (f MetricsFilter) filterGauge(gauge map[string]string) map[string]interface{} {
	data := make(map[string]interface{})
	data["type"] = "gauge"
	data["name"] = gauge["name"]
	data["unit"] = gauge["unit"]
	fValue, err := strconv.ParseFloat(gauge["value"], 64)
	if err != nil {
		data["value"] = gauge["value"]
	} else {
		data["value"] = fValue
	}
	return data
}

func (f MetricsFilter) filterCounter(counter map[string]string) map[string]interface{} {
	data := make(map[string]interface{})
	data["type"] = "counter"
	data["name"] = counter["name"]
	total, err := strconv.ParseUint(counter["total"], 10, 64)
	if err != nil {
		data["total"] = counter["total"]
	} else {
		data["total"] = total
	}
	delta, err := strconv.ParseUint(counter["delta"], 10, 64)
	if err != nil {
		data["delta"] = counter["delta"]
	} else {
		data["delta"] = delta
	}
	return data
}

func (f MetricsFilter) filterTimer(timer map[string]string) map[string]interface{} {
	data := make(map[string]interface{})
	data["type"] = "timer"
	data["name"] = timer["name"]
	start, err := strconv.ParseInt(timer["start"], 10, 64)
	if err != nil {
		data["start"] = timer["start"]
	} else {
		data["start"] = start
	}
	stop, err := strconv.ParseInt(timer["stop"], 10, 64)
	if err != nil {
		data["stop"] = timer["stop"]
	} else {
		data["stop"] = stop
	}
	return data
}

func (f MetricsFilter) Match(pMes *rfc5424.SyslogMessage) bool {
	return isMetrics(pMes)
}

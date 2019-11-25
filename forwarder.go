package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/orange-cloudfoundry/logs-service-broker/model"
	"github.com/orange-cloudfoundry/logs-service-broker/parser"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type Forwarder struct {
	sw     map[string]io.WriteCloser
	cacher *MetaCacher
	parser *parser.Parser
}

func NewForwarder(cacher *MetaCacher, writers map[string]io.WriteCloser, parsingKeys []model.ParsingKey) *Forwarder {
	return &Forwarder{
		sw:     writers,
		cacher: cacher,
		parser: parser.NewParser(parsingKeys),
	}
}

func (f Forwarder) Forward(bindingId string, rev int, message []byte) error {
	if len(message) == 0 {
		return nil
	}
	org, space, app := f.parser.ParseHostFromMessage(message)
	pLabels := prometheus.Labels{
		"instance_id": "",
		"binding_id":  bindingId,
		"plan_name":   "",
		"org":         org,
		"space":       space,
		"app":         app,
	}
	logData, err := f.cacher.LogMetadata(bindingId, rev, pLabels)
	if err != nil {
		logsSentFailure.With(pLabels).Inc()
		return err
	}
	pLabels["instance_id"] = logData.InstanceParam.InstanceID
	pLabels["plan_name"] = logData.InstanceParam.SyslogName
	timer := prometheus.NewTimer(logsSentDuration.With(pLabels))
	defer timer.ObserveDuration()

	patterns := make([]string, 0)
	if len(logData.InstanceParam.Patterns) > 0 {
		patterns = append(patterns, model.Patterns(logData.InstanceParam.Patterns).ToList()...)
	}

	timerParse := prometheus.NewTimer(logsParseDuration.With(pLabels))
	pMes, err := f.parser.Parse(logData, message, patterns...)
	if err != nil {
		logsSentFailure.With(pLabels).Inc()
		timerParse.ObserveDuration()
		return err
	}
	timerParse.ObserveDuration()
	if pMes == nil {
		logsSentFailure.With(pLabels).Inc()
		return nil
	}
	fMes, err := pMes.String()
	if err != nil {
		logsSentFailure.With(pLabels).Inc()
		return err
	}

	writer, err := f.foundWriter(logData.InstanceParam.SyslogName)
	if err != nil {
		logsSentFailure.With(pLabels).Inc()
		return err
	}

	logrus.
		WithField("binding_id", bindingId).
		WithField("org", org).
		WithField("space", space).
		WithField("app", app).
		Debug(fMes)

	_, err = writer.Write([]byte(fMes))
	if err != nil {
		logsSentFailure.With(pLabels).Inc()
		return err
	}

	logsSent.With(pLabels).Inc()
	return nil
}

func (f Forwarder) foundWriter(writerName string) (io.WriteCloser, error) {
	w, ok := f.sw[writerName]
	if !ok {
		return nil, fmt.Errorf("syslog '%s' not found", writerName)
	}
	return w, nil
}

func (f Forwarder) ForwardHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	v := mux.Vars(r)

	var bindingId string
	if _, ok := v["bindingId"]; ok {
		bindingId = v["bindingId"]
	}
	if bindingId == "" {
		hSplit := strings.Split(r.Host, ".")
		bindingId = hSplit[0]
	}

	rev := 0
	revStr, revExists := r.URL.Query()[model.RevKey]
	if revExists {
		var err error
		rev, err = strconv.Atoi(revStr[0])
		if err != nil {
			logrus.Warnf("Cannot convert rev for binding '%s' using rev 0, error is: %s", bindingId, err.Error())
		}
	}

	b, _ := ioutil.ReadAll(r.Body)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.WithField("binding_id", bindingId).Panic(r)
			}
		}()
		err := f.Forward(bindingId, rev, b)
		if err != nil {
			logrus.WithField("binding_id", bindingId).Error(err.Error())
		}
	}()
}

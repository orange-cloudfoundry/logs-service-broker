package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	"github.com/orange-cloudfoundry/logs-service-broker/model"
	"github.com/orange-cloudfoundry/logs-service-broker/parser"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

type Forwarder struct {
	sw     map[string]io.WriteCloser
	db     *gorm.DB
	parser *parser.Parser
}

func NewForwarder(db *gorm.DB, writers map[string]io.WriteCloser) *Forwarder {
	return &Forwarder{
		sw:     writers,
		db:     db,
		parser: parser.NewParser(),
	}
}

func (f Forwarder) Forward(bindingId string, message []byte) error {
	var logData model.LogMetadata
	f.db.Set("gorm:auto_preload", true).First(&logData, "binding_id = ?", bindingId)
	if logData.BindingID == "" {
		return fmt.Errorf("binding id '%s' not found", bindingId)
	}

	org, space, app := f.parser.ParseHostFromMessage(message)

	pLabels := prometheus.Labels{
		"instance_id": logData.InstanceParam.InstanceID,
		"binding_id":  logData.BindingID,
		"plan_name":   logData.InstanceParam.SyslogName,
		"org":         org,
		"space":       space,
		"app":         app,
	}
	timer := prometheus.NewTimer(logsSentDuration.With(pLabels))
	defer timer.ObserveDuration()

	patterns := make([]string, 0)
	if len(logData.InstanceParam.Patterns) > 0 {
		patterns = append(patterns, model.Patterns(logData.InstanceParam.Patterns).ToList()...)
	}
	if len(logData.Patterns) > 0 {
		patterns = append(patterns, model.Patterns(logData.Patterns).ToList()...)
	}

	pMes, err := f.parser.Parse(logData, message, patterns...)
	if err != nil {
		logsSentFailure.With(pLabels).Inc()
		return err
	}
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

	b, _ := ioutil.ReadAll(r.Body)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.WithField("binding_id", bindingId).Panic(r)
			}
		}()
		err := f.Forward(bindingId, b)
		if err != nil {
			logrus.WithField("binding_id", bindingId).Error(err.Error())
		}
	}()
}

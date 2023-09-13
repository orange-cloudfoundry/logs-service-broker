package api

import (
	"fmt"
	"github.com/orange-cloudfoundry/logs-service-broker/dbservices"
	"github.com/orange-cloudfoundry/logs-service-broker/metrics"
	"io"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/orange-cloudfoundry/logs-service-broker/model"
	"github.com/orange-cloudfoundry/logs-service-broker/parser"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type AuthorizeFunc = func(*http.Request) bool

type Forwarder struct {
	sw         map[string]io.WriteCloser
	cacher     dbservices.Cacher
	parser     *parser.Parser
	config     *model.ForwarderConfig
	authorizer AuthorizeFunc
}

// NewForwarder -
// 1. compute once for all the authorization function instead of switching at each requests
func NewForwarder(
	cacher *dbservices.MetaCacher,
	writers map[string]io.WriteCloser,
	config *model.Config,
) *Forwarder {
	f := &Forwarder{
		sw:         writers,
		cacher:     cacher,
		parser:     parser.NewParser(config.Forwarder.ParsingKeys, config.Forwarder.IgnoreTagsStructuredData),
		config:     &config.Forwarder,
		authorizer: alwaysAuthorized,
	}

	// 1.
	if len(config.Forwarder.AllowedHosts) != 0 {
		f.authorizer = f.isAuthorized
	}
	return f
}

func (f Forwarder) Forward(bindingID string, rev int, message []byte) error {
	if len(message) == 0 {
		return nil
	}
	org, space, app := f.parser.ParseHostFromMessage(message)
	labels := prometheus.Labels{
		"instance_id": "",
		"binding_id":  bindingID,
		"plan_name":   "",
		"org":         org,
		"space":       space,
		"app":         app,
	}

	meta, err := f.cacher.LogMetadata(bindingID, rev, labels)
	if err != nil {
		metrics.LogsSentFailure.With(labels).Inc()
		return err
	}
	labels["instance_id"] = meta.InstanceParam.InstanceID
	labels["plan_name"] = meta.InstanceParam.SyslogName

	// catch panic to prevent exit
	defer func() {
		if r := recover(); r != nil {
			logrus.WithField("binding_id", bindingID).Error(string(debug.Stack()))
			metrics.LogsSentFailure.With(labels).Inc()
		}
	}()

	timer := prometheus.NewTimer(metrics.LogsSentDuration.With(labels))
	defer timer.ObserveDuration()

	patterns := make([]string, 0)
	if len(meta.InstanceParam.Patterns) > 0 {
		patterns = append(patterns, model.Patterns(meta.InstanceParam.Patterns).ToList()...)
	}

	parsed, err := f.parser.Parse(meta, message, patterns)
	if err != nil {
		metrics.LogsSentFailure.With(labels).Inc()
		return err
	}

	if parsed == nil {
		metrics.LogsSentFailure.With(labels).Inc()
		return nil
	}
	fMes, err := parsed.String()
	if err != nil {
		metrics.LogsSentFailure.With(labels).Inc()
		return err
	}

	writer, err := f.foundWriter(meta.InstanceParam.SyslogName)
	if err != nil {
		metrics.LogsSentFailure.With(labels).Inc()
		return err
	}

	_, err = writer.Write([]byte(fMes))
	if err != nil {
		metrics.LogsSentFailure.With(labels).Inc()
		return err
	}

	metrics.LogsSent.With(labels).Inc()
	return nil
}

func (f Forwarder) foundWriter(writerName string) (io.WriteCloser, error) {
	w, ok := f.sw[writerName]
	if !ok {
		return nil, fmt.Errorf("syslog '%s' not found", writerName)
	}
	return w, nil
}

func alwaysAuthorized(r *http.Request) bool {
	return true
}

// isAuthorized -
// in: logservice.service.cf.internal:8089
func (f Forwarder) isAuthorized(r *http.Request) bool {
	hostname := strings.Split(r.Host, ":")[0]
	for _, host := range f.config.AllowedHosts {
		if hostname == host {
			return true
		}
	}
	return false
}

func (f Forwarder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if !f.authorizer(r) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(http.StatusText(http.StatusUnauthorized)))
		return
	}

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

	b, _ := io.ReadAll(r.Body)
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

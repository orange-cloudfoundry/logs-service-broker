package dbservices

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Cacher

import (
	"github.com/orange-cloudfoundry/logs-service-broker/model"
	"github.com/prometheus/client_golang/prometheus"
)

type Cacher interface {
	PreCache() error
	LogMetadata(bindingID string, revision int, promLabels prometheus.Labels) (*model.LogMetadata, error)
}

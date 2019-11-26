package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/orange-cloudfoundry/logs-service-broker/model"
	"github.com/prometheus/client_golang/prometheus"
)

const AlwaysUseCacheKey = "always"

type LogMetadataCached struct {
	model.LogMetadata
	ExpireAt time.Time
}

type MetaCacher struct {
	db            *gorm.DB
	mapBinding    *sync.Map
	cacheDuration time.Duration
}

func NewMetaCacher(db *gorm.DB, cacheDuration string) (*MetaCacher, error) {
	cacheDuration = strings.TrimSpace(strings.ToLower(cacheDuration))
	var cd time.Duration
	var err error
	if cacheDuration == "-1" || cacheDuration == AlwaysUseCacheKey {
		cd = -1
	} else {
		cd, err = time.ParseDuration(cacheDuration)
		if err != nil {
			return nil, err
		}
	}
	return &MetaCacher{
		db:            db,
		cacheDuration: cd,
		mapBinding:    &sync.Map{},
	}, nil
}

func (c *MetaCacher) LogMetadata(bindingId string, revision int, promLabels prometheus.Labels) (model.LogMetadata, error) {
	key := c.genKey(bindingId, revision)
	logCached, ok := c.mapBinding.Load(key)
	if ok && !c.mustEvict(logCached.(LogMetadataCached), revision) {
		return logCached.(LogMetadataCached).LogMetadata, nil
	}
	var logData model.LogMetadata
	c.db.First(&logData, "binding_id = ?", bindingId)
	if logData.BindingID == "" {
		return model.LogMetadata{}, fmt.Errorf("binding id '%s' not found", bindingId)
	}
	var instanceParam model.InstanceParam
	c.db.Set("gorm:auto_preload", true).First(&instanceParam, "instance_id = ? and revision = ?", logData.InstanceID, revision)
	logData.InstanceParam = instanceParam
	c.mapBinding.Store(key, LogMetadataCached{
		LogMetadata: logData,
		ExpireAt:    time.Now().Add(c.cacheDuration),
	})
	promLabels["instance_id"] = logData.InstanceParam.InstanceID
	promLabels["plan_name"] = logData.InstanceParam.SyslogName
	logsSentWithoutCache.With(promLabels).Inc()
	return logData, nil
}

func (c *MetaCacher) mustEvict(logCached LogMetadataCached, revision int) bool {
	if c.cacheDuration > 0 && logCached.ExpireAt.After(time.Now()) {
		return true
	}
	if logCached.InstanceParam.Revision != revision {
		return true
	}
	return false
}

func (c *MetaCacher) EvictByBindingId(bindingId string) {
	toDelete := make([]string, 0)
	c.mapBinding.Range(func(key, value interface{}) bool {
		if strings.HasPrefix(key.(string), bindingId) {
			toDelete = append(toDelete, key.(string))
		}
		return true
	})
	for _, del := range toDelete {
		c.mapBinding.Delete(del)
	}
}

func (c *MetaCacher) genKey(bindingId string, revision int) string {
	return fmt.Sprintf("%s~%d", bindingId, revision)
}

// clean expired cached to ensure to not use to much memory
// This need to be called in a goroutine and do a kind of stop the world during cleaning sync map
func (c *MetaCacher) Cleaner() {
	sleepDuration := 24 * time.Hour
	if c.cacheDuration > 0 {
		sleepDuration = c.cacheDuration
	}
	for {
		c.cleanExpired()
		c.cleanWhenNotInDB()
		time.Sleep(sleepDuration)
	}
}

func (c *MetaCacher) cleanWhenNotInDB() {
	if c.cacheDuration > 0 {
		return
	}
	c.mapBinding.Range(func(key, value interface{}) bool {
		var logData model.LogMetadata
		c.db.First(&logData, "binding_id = ?", value.(LogMetadataCached).BindingID)
		if logData.BindingID != "" {
			return true
		}
		c.EvictByBindingId(logData.BindingID)
		return true
	})
}

func (c *MetaCacher) cleanExpired() {
	if c.cacheDuration < 0 {
		return
	}
	toDelete := make([]string, 0)
	now := time.Now()
	c.mapBinding.Range(func(key, value interface{}) bool {
		logData := value.(LogMetadataCached)
		if logData.ExpireAt.After(now) {
			toDelete = append(toDelete, key.(string))
		}
		return true
	})
	for _, del := range toDelete {
		c.mapBinding.Delete(del)
	}
}

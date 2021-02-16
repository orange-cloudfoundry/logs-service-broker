package dbservices

import (
	"fmt"
	"github.com/orange-cloudfoundry/logs-service-broker/metrics"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

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

func (c *MetaCacher) PreCache() error {
	metadatas := make([]model.LogMetadata, 0)
	err := c.db.Preload("InstanceParam", func(db *gorm.DB) *gorm.DB {
		return db.Set("gorm:auto_preload", true).Order("revision desc")
	}).Find(&metadatas).Error
	for _, meta := range metadatas {
		key := c.genKey(meta.BindingID, meta.InstanceParam.Revision)
		entry := LogMetadataCached{
			LogMetadata: meta,
			ExpireAt:    time.Now().Add(c.cacheDuration),
		}
		c.mapBinding.Store(key, &entry)
	}
	return err
}

func (c *MetaCacher) LogMetadata(
	bindingID string,
	revision int,
	promLabels prometheus.Labels,
) (*model.LogMetadata, error) {

	key := c.genKey(bindingID, revision)
	iEntry, ok := c.mapBinding.Load(key)
	if ok {
		entry := iEntry.(*LogMetadataCached)
		if !c.mustEvict(entry, revision) {
			return &(entry.LogMetadata), nil
		}
	}

	var (
		meta  model.LogMetadata
		param model.InstanceParam
	)

	if err := c.db.First(&meta, "binding_id = ?", bindingID).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return nil, fmt.Errorf("binding id '%s' not found", bindingID)
		}
		return nil, fmt.Errorf("unexpected DB error while fetching binding id '%s': %s", bindingID, err.Error())
	}

	err := c.db.Set("gorm:auto_preload", true).
		First(&param, "instance_id = ? and revision = ?", meta.InstanceID, revision).
		Error
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			err = fmt.Errorf("instance param '%s' with revision '%d' not found", meta.InstanceID, revision)
			return nil, err
		}
		err = fmt.Errorf("unexpected DB error while fetching instance param '%s/%d': %s", meta.InstanceID, revision, err.Error())
		return nil, err
	}

	meta.InstanceParam = param
	entry := LogMetadataCached{
		LogMetadata: meta,
		ExpireAt:    time.Now().Add(c.cacheDuration),
	}
	c.mapBinding.Store(key, &entry)

	promLabels["instance_id"] = meta.InstanceParam.InstanceID
	promLabels["plan_name"] = meta.InstanceParam.SyslogName
	metrics.LogsSentWithoutCache.With(promLabels).Inc()
	return &(entry.LogMetadata), nil
}

func (c *MetaCacher) mustEvict(entry *LogMetadataCached, revision int) bool {
	if c.cacheDuration > 0 && entry.ExpireAt.After(time.Now()) {
		return true
	}
	if entry.InstanceParam.Revision != revision {
		return true
	}
	return false
}

func (c *MetaCacher) evictByBindingID(bindingID string) {
	toDelete := make([]string, 0)
	c.mapBinding.Range(func(key, value interface{}) bool {
		if strings.HasPrefix(key.(string), bindingID) {
			toDelete = append(toDelete, key.(string))
		}
		return true
	})
	for _, del := range toDelete {
		c.mapBinding.Delete(del)
	}
}

func (c *MetaCacher) genKey(bindingID string, revision int) string {
	return fmt.Sprintf("%s~%d", bindingID, revision)
}

// Cleaner -
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

	cleanFunctor := func(key, iEntry interface{}) bool {
		var meta model.LogMetadata
		entry, _ := iEntry.(*LogMetadataCached)
		err := c.db.First(&meta, "binding_id = ?", entry.BindingID).Error
		if err == nil {
			return true
		}
		if gorm.IsRecordNotFoundError(err) {
			c.evictByBindingID(entry.BindingID)
			return true
		}
		log.Errorf("skipped db error: %s", err.Error())
		return true
	}
	c.mapBinding.Range(cleanFunctor)
}

func (c *MetaCacher) cleanExpired() {
	if c.cacheDuration < 0 {
		return
	}
	toDelete := make([]string, 0)
	now := time.Now()
	c.mapBinding.Range(func(key, iEntry interface{}) bool {
		entry := iEntry.(*LogMetadataCached)
		if entry.ExpireAt.After(now) {
			toDelete = append(toDelete, key.(string))
		}
		return true
	})
	for _, del := range toDelete {
		c.mapBinding.Delete(del)
	}
}

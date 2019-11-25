package main

import (
	"github.com/jinzhu/gorm"
	"github.com/orange-cloudfoundry/logs-service-broker/model"
	log "github.com/sirupsen/logrus"
	"gopkg.in/gormigrate.v1"
)

var labelsMigrated = false
var gormMigration = []*gormigrate.Migration{
	{
		ID: "init",
		Migrate: func(db *gorm.DB) error {
			db.AutoMigrate(&model.SourceLabel{})
			err := migrateLabels(db)
			if err != nil {
				return err
			}
			labelsMigrated = true
			db.AutoMigrate(&model.LogMetadata{}, &model.InstanceParam{}, &model.Patterns{}, &model.Label{})
			return nil
		},
		Rollback: func(db *gorm.DB) error {
			return nil
		},
	},
	{
		ID:      "migrate-labels",
		Migrate: migrateLabels,
		Rollback: func(db *gorm.DB) error {
			return nil
		},
	},
	{
		ID: "set-revision",
		Migrate: func(db *gorm.DB) error {
			var instanceParams []model.InstanceParam
			db.Where("revision IS NULL").Find(&instanceParams)
			for _, instanceParam := range instanceParams {
				db.Table("instance_params").
					Where("instance_id = ?", instanceParam.InstanceID).
					Update("revision", 0)
			}
			return nil
		},
		Rollback: func(db *gorm.DB) error {
			return nil
		},
	},
	{
		ID: "migrate-pm-instance",
		Migrate: func(db *gorm.DB) error {
			return db.Exec("ALTER TABLE instance_params DROP PRIMARY KEY, ADD PRIMARY KEY(instance_id, revision)").Error
		},
		Rollback: func(db *gorm.DB) error {
			return nil
		},
	},
}

func migrateLabels(db *gorm.DB) error {
	if !db.HasTable(&model.Label{}) || labelsMigrated {
		return nil
	}
	var labels []struct {
		BindingID string
		ID        string
	}

	db.Table("labels").Where("binding_id IS NOT NULL AND binding_id != '' ").Find(&labels)
	if len(labels) == 0 {
		return nil
	}
	bindMap := make(map[string][]string)
	for _, label := range labels {
		if v, ok := bindMap[label.BindingID]; ok {
			v = append(v, label.ID)
			bindMap[label.BindingID] = v
			continue
		}
		bindMap[label.BindingID] = []string{label.ID}
	}

	for bindingId, labelIds := range bindMap {
		var logMeta model.LogMetadata
		db.First(&logMeta, "binding_id = ?", bindingId)
		if logMeta.BindingID == "" || logMeta.InstanceID == "" {
			continue
		}
		for _, labelId := range labelIds {
			db.Model(&model.Label{}).Where("id = ?", labelId).Update("instance_id", logMeta.InstanceID)
		}
	}
	db.Model(&model.Label{}).DropColumn("binding_id")
	db.Model(&model.Pattern{}).DropColumn("binding_id")
	db.Delete(&model.Label{}, "instance_id IS NULL or instance_id = ''")
	db.Delete(&model.Pattern{}, "instance_id IS NULL or instance_id = ''")
	return nil
}

func migratePatternIfNeeded(db *gorm.DB, syslogAddrs model.SyslogAddresses) {
	log.Info("Migrating patterns if needed ...")
	for _, syslogAddr := range syslogAddrs {
		for _, pattern := range syslogAddr.Patterns {
			ists := make([]model.InstanceParam, 0)
			db.Table("instance_params i").
				Where("i.syslog_name = ? AND ? NOT IN (?)",
					syslogAddr.Name, pattern,
					db.Table("patterns p").Select("p.pattern").Where("p.instance_id = i.instance_id").QueryExpr(),
				).
				Find(&ists)
			if len(ists) == 0 {
				continue
			}
			entry := log.WithField("syslog_name", syslogAddr.Name).WithField("pattern", pattern)
			entry.Infof("Migrating %d instances to add this pattern", len(ists))
			for _, ist := range ists {
				db.Create(&model.Pattern{
					InstanceID: ist.InstanceID,
					Pattern:    pattern,
				})
			}
		}
	}
	log.Info("Finished migrating patterns if needed.")
}

package main

import (
	"github.com/jinzhu/gorm"
	"github.com/orange-cloudfoundry/logs-service-broker/model"
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
		db.Set("gorm:auto_preload", true).First(&logMeta, "binding_id = ?", bindingId)
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

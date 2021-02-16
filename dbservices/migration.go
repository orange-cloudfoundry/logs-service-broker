package dbservices

import (
	"fmt"

	"github.com/jinzhu/gorm"
	"github.com/orange-cloudfoundry/logs-service-broker/model"
	"gopkg.in/gormigrate.v1"
)

var labelsMigrated = false

type Migration struct {
	ID       string
	Migrate  func(db *gorm.DB, config *model.Config) error
	Rollback func(db *gorm.DB, config *model.Config) error
}

type Migrations struct {
	Config     *model.Config
	Migrations []*Migration
}

func (m Migrations) ToGormMigrate() []*gormigrate.Migration {
	finalMigrations := make([]*gormigrate.Migration, 0)
	for _, migration := range m.Migrations {
		finalMigrations = append(finalMigrations, &gormigrate.Migration{
			ID: migration.ID,
			Migrate: func(db *gorm.DB) error {
				return migration.Migrate(db, m.Config)
			},
			Rollback: func(db *gorm.DB) error {
				return migration.Rollback(db, m.Config)
			},
		})
	}
	return finalMigrations
}

func GormMigration() []*Migration {
	return []*Migration{
		{
			ID: "init",
			Migrate: func(db *gorm.DB, config *model.Config) error {
				fmt.Println("toto")
				err := db.AutoMigrate(&model.SourceLabel{}).Error
				if err != nil {
					return err
				}
				err = migrateLabels(db, config)
				if err != nil {
					return err
				}
				labelsMigrated = true
				err = db.AutoMigrate(&model.LogMetadata{}, &model.InstanceParam{}, &model.Patterns{}, &model.Label{}).Error
				if err != nil {
					return err
				}
				return nil
			},
			Rollback: func(db *gorm.DB, config *model.Config) error {
				return nil
			},
		},
		{
			ID:      "migrate-labels",
			Migrate: migrateLabels,
			Rollback: func(db *gorm.DB, config *model.Config) error {
				return nil
			},
		},
		{
			ID: "set-revision",
			Migrate: func(db *gorm.DB, config *model.Config) error {
				var instanceParams []model.InstanceParam
				db.Where("revision IS NULL").Find(&instanceParams)
				for _, instanceParam := range instanceParams {
					db.Table("instance_params").
						Where("instance_id = ?", instanceParam.InstanceID).
						Update("revision", 0)
				}
				return nil
			},
			Rollback: func(db *gorm.DB, config *model.Config) error {
				return nil
			},
		},
		{
			ID: "migrate-pm-instance",
			Migrate: func(db *gorm.DB, config *model.Config) error {
				return db.Exec("ALTER TABLE instance_params DROP PRIMARY KEY, ADD PRIMARY KEY(instance_id, revision)").Error
			},
			Rollback: func(db *gorm.DB, config *model.Config) error {
				return nil
			},
		},
		{
			ID: "add-usetls-and-draintype",
			Migrate: func(db *gorm.DB, config *model.Config) error {
				err := db.AutoMigrate(&model.InstanceParam{}).Error
				if err != nil {
					return err
				}
				ists := make([]model.InstanceParam, 0)
				db.Find(&ists)
				for _, ist := range ists {
					db.Table("instance_params").
						Where("instance_id = ? and revision = ?", ist.InstanceID, ist.Revision).
						Updates(map[string]interface{}{"use_tls": true, "drain_type": ""})
				}
				return nil
			},
			Rollback: func(db *gorm.DB, config *model.Config) error {
				return nil
			},
		},
	}
}

func migrateLabels(db *gorm.DB, config *model.Config) error {
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

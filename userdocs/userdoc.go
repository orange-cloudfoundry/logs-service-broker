package userdocs

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"

	"github.com/gobuffalo/packr/v2"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	"github.com/orange-cloudfoundry/logs-service-broker/model"
)

type UserDoc struct {
	db     *gorm.DB
	config model.Config
}

var boxTemplates = packr.New("userdocs_templates", "./templates")
var mainTpl *template.Template

func NewUserDoc(db *gorm.DB, config model.Config) *UserDoc {
	var err error
	mainFile, _ := boxTemplates.FindString("main.html")
	mainTpl, err = template.New("main.html").Funcs(tplfuncs).Parse(mainFile)
	if err != nil {
		panic(fmt.Sprintf("Cannot parse template 'main.html': %s", err.Error()))
	}
	for _, tplName := range boxTemplates.List() {
		if tplName == "main.html" {
			continue
		}
		tplTxt, _ := boxTemplates.FindString(tplName)
		_, err := mainTpl.New(tplName).Funcs(tplfuncs).Parse(tplTxt)
		if err != nil {
			panic(fmt.Sprintf("Cannot parse template '%s': %s", tplName, err.Error()))
		}
	}
	return &UserDoc{
		db:     db,
		config: config,
	}
}

func (d UserDoc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	v := mux.Vars(r)

	var instanceId string
	if _, ok := v["instanceId"]; ok {
		instanceId = v["instanceId"]
	}
	var instanceParam *model.InstanceParam
	logMetadatas := make([]model.LogMetadata, 0)
	if instanceId != "" {
		instanceParam = &model.InstanceParam{}
		d.db.Set("gorm:auto_preload", true).Order("revision desc").First(instanceParam, "instance_id = ?", instanceId)
		d.db.Find(&logMetadatas, "instance_id = ?", instanceId)

	}
	buf := &bytes.Buffer{}
	err := mainTpl.Execute(buf, struct {
		Config        model.Config
		InstanceParam *model.InstanceParam
		LogMetadatas  []model.LogMetadata
	}{d.config, instanceParam, logMetadatas})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.Write(buf.Bytes())
}

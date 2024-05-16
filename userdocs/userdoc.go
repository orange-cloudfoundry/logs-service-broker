package userdocs

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	"github.com/orange-cloudfoundry/logs-service-broker/model"
)

type UserDoc struct {
	db     *gorm.DB
	config *model.Config
}

var (
	// embeddedUserDocMain holds our main.html
	//
	//go:embed templates/main.html
	embeddedUserDocMain string
)

// embeddedUserDocTemplates holds our Markdown files
//
//go:embed templates/*.md
var embeddedUserDocTemplates embed.FS

var mainTpl *template.Template

func NewUserDoc(db *gorm.DB, config *model.Config) *UserDoc {
	var err error
	mainTpl, err = template.New("main.html").Funcs(tplfuncs).Parse(embeddedUserDocMain)
	if err != nil {
		panic(fmt.Sprintf("Cannot parse template 'main.html': %s", err.Error()))
	}
	templatesDirectory, err := fs.Sub(embeddedUserDocTemplates, "templates")
	if err != nil {
		panic("unable to sub embedded templates dir: " + err.Error())
	}
	content, err := fs.ReadDir(templatesDirectory, ".")
	if err != nil {
		panic("unable to read embedded templates dir: " + err.Error())
	}
	for _, file := range content {
		tplName := file.Name()
		tplFile, err := fs.ReadFile(templatesDirectory, tplName)
		if err != nil {
			panic(fmt.Sprintf("unable to read file '%s' from '%s': %s", tplName, templatesDirectory, err))
		}
		_, err = mainTpl.New(tplName).Funcs(tplfuncs).Parse(string(tplFile))
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
	}{*d.config, instanceParam, logMetadatas})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		// nolint:errcheck
		w.Write([]byte(err.Error()))
		return
	}
	// nolint:errcheck
	w.Write(buf.Bytes())
}

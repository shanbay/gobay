package main

import (
	"bytes"
	"github.com/iancoleman/strcase"
	"github.com/markbates/pkger"
	"github.com/spf13/cobra"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"text/template"
)

type _projTemplate struct {
	content []byte
	dstPath string
	skip    bool
	mode    os.FileMode
}

type _projDir struct {
	dstPath string
	mode    os.FileMode
}

type _projConfig struct {
	Url   string
	Name  string
	Skips []string
}

var (
	projDirs      = []_projDir{}
	projTemplates = []_projTemplate{}
	projConfig    = _projConfig{}
	projRawTmpl   = map[string]string{}
	tmplFuncs     = template.FuncMap{
		"toCamel":      strcase.ToCamel,
		"toLowerCamel": strcase.ToLowerCamel,
		"toSnake":      strcase.ToSnake,
	}
)

const (
	TMPLSUFFIX               = ".tmpl"
	RAW_TMPL_DIR             = "enttmpl"
	DIRMODE      os.FileMode = os.ModeDir | 0755
	FILEMODE     os.FileMode = 0644
	DIRPREFIX                = "/cmd/gobay/templates/"
	TRIMPREFIX               = "github.com/shanbay/gobay:/cmd/gobay/templates/"
)

func main() {
	cmd := &cobra.Command{Use: "gobay"}
	cmdNew := &cobra.Command{
		Use: "new [projectURL]",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 1 {
				check(cmd.Help())
				return
			}
			url := args[0]
			url = strings.TrimSuffix(url, "/")
			projConfig.Url = url
			if projConfig.Name == "" {
				strs := strings.Split(url, "/")
				projConfig.Name = strs[len(strs)-1]
			}
			newProject()
		},
		Short: "initialize new gobay project",
		Long:  "Example: `gobay new github.com/shanbay/project`",
	}
	cmdNew.Flags().StringVar(&projConfig.Name, "name", "", "specific project name")
	cmdNew.Flags().StringSliceVar(&projConfig.Skips, "skip", nil, "skip templates")

	cmd.AddCommand(cmdNew)
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}

func newProject() {
	if err := os.Mkdir(projConfig.Name, DIRMODE); os.IsExist(err) {
		log.Fatalf("already exists: %v", projConfig.Name)
	}

	// load
	if err := loadTemplates(); err != nil {
		log.Fatalln(err)
	}

	// render
	renderTemplates()

	// copy
	copyTmplFiles()
}

// loadTemplates loads templates and directory structure.
func loadTemplates() error {
	if err := pkger.Walk(
		DIRPREFIX,
		func(filePath string, info os.FileInfo, err error) error {
			if err != nil || len(filePath) <= len(TRIMPREFIX) { // dir `templates`
				return err
			}
			targetPath := path.Join(
				projConfig.Name,
				strings.TrimPrefix(filePath, TRIMPREFIX),
			)
			// dir
			if info.IsDir() {
				projDirs = append(projDirs, _projDir{
					dstPath: targetPath,
					mode:    info.Mode(),
				})
				return nil
			}

			// file
			if strings.Contains(targetPath, RAW_TMPL_DIR) { // enttmpl
				projRawTmpl[filePath] = targetPath
				return nil
			}
			file, err := pkger.Open(filePath)
			if err != nil {
				return err
			}
			b := make([]byte, info.Size())
			if _, err = file.Read(b); err != nil {
				return err
			}
			projTemplates = append(projTemplates, _projTemplate{
				content: b,
				dstPath: strings.TrimSuffix(targetPath, TMPLSUFFIX),
				mode:    info.Mode(),
			})
			return nil
		},
	); err != nil {
		return err
	}
	return nil
}

func renderTemplates() {
	// dir
	for _, dir := range projDirs {
		check(os.MkdirAll(dir.dstPath, dir.mode))
	}

	// file
	gobayTmpl := template.New("gobay")
	gobayTmpl.Funcs(tmplFuncs)
	for _, f := range projTemplates {
		if f.skip {
			continue
		}
		tmpl := template.Must(gobayTmpl.Parse(string(f.content)))
		b := bytes.NewBuffer(nil)
		if err := tmpl.Execute(b, projConfig); err != nil {
			log.Fatalln(err)
		}
		if err := ioutil.WriteFile(f.dstPath, b.Bytes(), f.mode); err != nil {
			log.Fatalln(err)
		}
	}
}

// copyTmplFiles copys .tpml file(like ent templates).
func copyTmplFiles() {
	for sourcePath, targetPath := range projRawTmpl {
		file, err := pkger.Open(sourcePath)
		if err != nil {
			panic(err)
		}
		info, err := file.Stat()
		if err != nil {
			panic(err)
		}
		b := make([]byte, info.Size())
		if _, err := file.Read(b); err != nil {
			panic(err)
		}
		check(ioutil.WriteFile(targetPath, b, FILEMODE))
	}
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

package main

import (
	"bytes"
	"embed"
	"io/fs"
	"log"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/iancoleman/strcase"
	"github.com/spf13/cobra"
)

//go:embed templates/*
var templatesDir embed.FS

type _projTemplate struct {
	content []byte
	dstPath string
	mode    os.FileMode
}

type _projDir struct {
	dstPath string
	mode    os.FileMode
}

type _projConfig struct {
	Url            string
	Name           string
	SkipSentry     bool
	SkipAsyncTask  bool
	SkipCache      bool
	SkipElasticApm bool
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
	DIRPREFIX                = "."
	TRIMPREFIX               = "templates"
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
	cmdNew.Flags().BoolVar(&projConfig.SkipSentry, "skip-sentry", false, "skip sentry")
	cmdNew.Flags().BoolVar(&projConfig.SkipElasticApm, "skip-elasticapm", false, "skip elastic APM")
	cmdNew.Flags().BoolVar(&projConfig.SkipCache, "skip-cache", false, "skip cache")
	cmdNew.Flags().BoolVar(&projConfig.SkipAsyncTask, "skip-asynctask", false, "skip asynctask")

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
	if err := fs.WalkDir(templatesDir, DIRPREFIX,
		func(filePath string, entry fs.DirEntry, err error) error {
			if err != nil || len(filePath) <= 1 { // dir `templates`
				return err
			}
			targetPath := path.Join(
				projConfig.Name,
				strings.TrimPrefix(filePath, TRIMPREFIX),
			)
			info, err := entry.Info()
			if err != nil {
				return err
			}
			// dir
			if entry.IsDir() {
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
			b, err := fs.ReadFile(templatesDir, filePath)
			if err != nil {
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
		if projConfig.SkipAsyncTask && strings.Contains(dir.dstPath, "asynctask") {
			continue
		}
		check(os.MkdirAll(dir.dstPath, DIRMODE))
	}

	// file
	gobayTmpl := template.New("gobay")
	gobayTmpl.Funcs(tmplFuncs)
	for _, f := range projTemplates {
		tmpl := template.Must(gobayTmpl.Parse(string(f.content)))
		b := bytes.NewBuffer(nil)
		if err := tmpl.Execute(b, projConfig); err != nil {
			log.Fatalln(err)
		}
		// empty file
		if b.Len() <= 1 {
			continue
		}
		if err := os.WriteFile(f.dstPath, b.Bytes(), FILEMODE); err != nil {
			log.Fatalln(err)
		}
	}
}

// copyTmplFiles copys .tpml file(like ent templates).
func copyTmplFiles() {
	for sourcePath, targetPath := range projRawTmpl {
		file, err := templatesDir.Open(sourcePath)
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
		check(os.WriteFile(targetPath, b, FILEMODE))
	}
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

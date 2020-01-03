package main

import (
	"io/ioutil"
	"bytes"
	"github.com/markbates/pkger"
	"github.com/spf13/cobra"
	"log"
	"os"
	"path"
	"strings"
	"text/template"
)

func main() {
	cmd := &cobra.Command{Use: "gobay"}
	cmdNew := &cobra.Command{
		Use: "new [projectURL]",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 1 {
				cmd.Help()
				return
			}
			url := args[0]
			if projConfig.name == "" {
				strs := strings.Split(url, "/")
				projConfig.name = strs[len(strs)-1]
			}
			newProject(url)
		},
		Short: "initialize new gobay project",
		Long:  "Example: `gobay new github.com/shanbay/project`",
	}
	cmdNew.Flags().StringVar(&projConfig.name, "name", "", "specific project name")
	cmdNew.Flags().StringSliceVar(&projConfig.skips, "skip", nil, "skip templates")

	cmd.AddCommand(cmdNew)
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}

func newProject(url string) {
	if err := os.Mkdir(projConfig.name, DIRMODE); os.IsExist(err) {
		log.Fatalf("already exists: %v", projConfig.name)
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
			if err != nil || len(filePath) <= len(TRIMPREFIX) {
				return err
			}
			targetPath := path.Join(projConfig.name, filePath[len(TRIMPREFIX):])
			// dir
			if info.IsDir() {
				projDirs = append(projDirs, _projDir{
					dstPath: targetPath,
					mode:    info.Mode(),
				})
				return nil
			}

			// file
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
				dstPath: targetPath,
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
	log.Println(projDirs)
	log.Println(projTemplates)
	for _, dir := range projDirs {
		os.MkdirAll(dir.dstPath, dir.mode)
	}

	// file
	gobayTmpl := template.New("gobay")
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
func copyTmplFiles() {}

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
	url   string
	name  string
	skips []string
}

var (
	projDirs      = []_projDir{}
	projTemplates = []_projTemplate{}
	projConfig    = _projConfig{}
)

const (
	DIRMODE    os.FileMode = os.ModeDir | 0755
	DIRPREFIX              = "/cmd/gobay/templates/"
	TRIMPREFIX             = "github.com/shanbay/gobay:/cmd/gobay/templates/"
)

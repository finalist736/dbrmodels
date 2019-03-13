package main

import (
	"database/sql"
	"fmt"
	"github.com/serenize/snaker"
	"os"
	"os/exec"
	"strings"
	"text/template"
)

const data = `package {{.PkgName}}
{{if .DbrUsed}}import "github.com/gocraft/dbr"{{end}}

const TableName{{.Table}} = "{{.TableOrigin}}"
var fieldsNames{{.Table}} = []string{ {{.FieldsNames}} }
{{if .AutoInc}}var autoIncrementField{{.Table}} = "{{.AutoInc}}"{{end}}


type DB{{.Table}} struct {
	{{range .Fields}}{{.Name}} {{.Type}} {{.Tag}}
	{{end}}
}

func NewDB{{.Table}}() *DB{{.Table}} {
	return new(DB{{.Table}})
}

func NewSliceDB{{.Table}}() []*DB{{.Table}} {
	return make([]*DB{{.Table}}, 0)
}

func FieldsNames{{.Table}}() []string {
	return fieldsNames{{.Table}}
}
{{if .AutoInc}}
func FieldsNamesWithOutAI{{.Table}}() []string {
	var slice []string
	for _, iterator := range fieldsNames{{.Table}} {
		if iterator == autoIncrementField{{.Table}} {
			continue
		}
		slice = append(slice, iterator)
	}
	return slice
}

{{end}}
`

type Field struct {
	Name string
	Type string
	Tag  string
}

type TplData struct {
	PkgName     string
	TableOrigin string
	Table       string
	Fields      []Field
	FieldsNames string
	AutoInc     string
	DbrUsed     bool
}

func CreateTableModel(path, table, projectname string, db *sql.DB, verbose bool) {
	var (
		name  string
		typ   string
		null  string
		key   string
		def   sql.NullString
		extra string
	)

	templateData := TplData{}
	templateData.PkgName = projectname
	templateData.TableOrigin = table
	templateData.Table = strings.Title(table)

	// get table columns info
	q := fmt.Sprintf("SHOW COLUMNS FROM %s", table)
	if rows, err := db.Query(q); err == nil {
		defer rows.Close()
		if verbose {
			fmt.Println("\tfields:")
		}
		for rows.Next() {
			err := rows.Scan(&name, &typ, &null, &key, &def, &extra)
			if err != nil {
				fmt.Fprintf(os.Stderr, "rows.Scan: %s\n", err.Error())
				continue
			}
			if verbose {
				fmt.Printf("\t\tname: `%s` type: %s null: %s key: %s def: %s extra: %s\n", name, typ, null, key, def.String, extra)
			}
			titled_name := strings.Title(name)
			if extra == "auto_increment" {
				templateData.AutoInc = name
			}

			if strings.Contains(typ, "enum") {
				if null == "YES" {
					templateData.DbrUsed = true
					typ = "dbr.NullString"
				} else {
					typ = "string"
				}
			}
			if typ == "tinyint(1)" { // bool need be first because next `strings.Contains(typ, "int")`
				if null == "YES" {
					templateData.DbrUsed = true
					typ = "dbr.NullBool"
				} else {
					typ = "bool"
				}
			} else if strings.Contains(typ, "int") {
				if null == "YES" {
					templateData.DbrUsed = true
					typ = "dbr.NullInt64"
				} else {
					typ = "int64"
				}
			} else if strings.Contains(typ, "float") ||
				strings.Contains(typ, "decimal") ||
				strings.Contains(typ, "double") ||
				strings.Contains(typ, "real") {
				if null == "YES" {
					templateData.DbrUsed = true
					typ = "dbr.NullFloat64"
				} else {
					typ = "float64"
				}
			} else if strings.Contains(typ, "date") || strings.Contains(typ, "timestamp") {
				templateData.DbrUsed = true
				typ = "dbr.NullTime"
			} else {
				if null == "YES" {
					templateData.DbrUsed = true
					typ = "dbr.NullString"
				} else {
					typ = "string"
				}
			}

			tag := fmt.Sprintf("`db:\"%s\" json:\"%s\"`", name, snaker.CamelToSnake(name))
			if verbose {
				fmt.Printf("\t\t\t => %s %s %s\n", titled_name, typ, tag)
			}
			table_field := Field{titled_name, typ, tag}
			templateData.Fields = append(templateData.Fields, table_field)
			templateData.FieldsNames = fmt.Sprintf("%s, \"%s\"", templateData.FieldsNames, name)
		}
		templateData.FieldsNames = strings.Trim(templateData.FieldsNames, ",")
	}
	t := template.Must(template.New("struct").Parse(data))

	//fullPath := path + "/" + table
	fullFileName := path + "/db_" + table + ".go"
	//	err := os.MkdirAll(fullPath, 0700)
	//	if err != nil {
	//		fmt.Fprintf(os.Stderr, "file creating error: %s", err)
	//		return
	//	}

	file, err := os.Create(fullFileName)
	defer file.Close()
	if err == nil {
		if err := t.Execute(file, templateData); err != nil {
			fmt.Fprintf(os.Stderr, "template executing: %s", err)
			return
		}
		cmd := exec.Command("go", "fmt", fullFileName)
		err = cmd.Start()
		if err == nil {
			err = cmd.Wait()
		}
	}
}

package main

import (
	"os"
	"text/template"
)

type Release struct {
	Release     string
	PackageName string
}

type Data struct {
	Releases []Release
}

func main() {
	data := Data{
		Releases: []Release{
			{Release: "R4", PackageName: "r4"},
			{Release: "R4B", PackageName: "r4b"},
			{Release: "R5", PackageName: "r5"},
		},
	}

	// Template and output are in the same directory as the go:generate directive (internal/)
	tmpl, err := template.ParseFiles("out.go.tmpl")
	if err != nil {
		panic(err)
	}

	f, err := os.Create("generated.go")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	err = tmpl.Execute(f, data)
	if err != nil {
		panic(err)
	}
}

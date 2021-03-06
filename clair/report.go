package clair

import (
	"bytes"
	"fmt"
	"math"
	"text/template"
	"strings"

	"github.com/coreos/clair/api/v1"
	"github.com/coreos/clair/utils/types"
	"github.com/spf13/viper"
	"os"
)

//execute go generate ./clair
//go:generate go-bindata -pkg clair -o templates.go templates/...

//Report Reporting Config value
var Report ReportConfig

//ReportConfig  Reporting configuration
type ReportConfig struct {
	Path   string
	Format string
}

//ReportAsHTML report analysis as HTML
func ReportAsHTML(analyzes ImageAnalysis) (string, error) {
	asset, err := Asset("templates/analysis-template.html")
	if err != nil {
		return "", fmt.Errorf("accessing template: %v", err)
	}

	funcs := template.FuncMap{
		"vulnerabilities":       vulnerabilities,
		"allVulnerabilities":    AllVulnerabilities,
		"sortedVulnerabilities": sortedVulnerabilities,
	}

	templte := template.Must(template.New("analysis-template").Funcs(funcs).Parse(string(asset)))

	var doc bytes.Buffer
	err = templte.Execute(&doc, analyzes)
	if err != nil {
		return "", fmt.Errorf("rendering HTML report: %v", err)
	}
	return doc.String(), nil
}

func invertedPriorities() []types.Priority {
	ip := make([]types.Priority, len(types.Priorities))
	for i, j := 0, len(types.Priorities)-1; i <= j; i, j = i+1, j-1 {
		ip[i], ip[j] = types.Priorities[j], types.Priorities[i]
	}
	return ip

}

type vulnerabilityWithFeature struct {
	v1.Vulnerability
	Feature string
}

//VulnerabiliesCounts Total count of vulnerabilities by type
type vulnerabiliesCounts map[types.Priority]int

//Total return to total of Vulnerabilities
func (v vulnerabiliesCounts) Total() int {
	var c int
	for _, count := range v {
		c += count
	}
	return c
}

//Count return count of severities in Vulnerabilities
func (v vulnerabiliesCounts) Count(severity string) int {
	return v[types.Priority(severity)]
}

//RelativeCount get the percentage of vulnerabilities of a severity
func (v vulnerabiliesCounts) RelativeCount(severity string) float64 {
	count := v[types.Priority(severity)]
	result := float64(count) / float64(v.Total()) * 100
	return math.Ceil(result*100) / 100
}

// allVulnerabilities Total count of vulnerabilities
func AllVulnerabilities(imageAnalysis ImageAnalysis) vulnerabiliesCounts {
	result := make(vulnerabiliesCounts)

	l := imageAnalysis.MostRecentLayer()
	vulnerabilities := make(map[string]string)

	for _, f := range l.Layer.Features {
		for _, v := range f.Vulnerabilities {
			if _, exist := vulnerabilities[v.Name]; !exist {
				vulnerabilities[v.Name] = v.Severity
			}
		}
	}
	for _, p := range invertedPriorities() {
		for _, v := range vulnerabilities {
			if types.Priority(v) == p {
				result[p]++
			}
		}
	}

	return result
}

//Vulnerabilities return a list a vulnerabilities
func vulnerabilities(imageAnalysis ImageAnalysis) map[types.Priority][]vulnerabilityWithFeature {

	result := make(map[types.Priority][]vulnerabilityWithFeature)

	l := imageAnalysis.MostRecentLayer()
	for _, f := range l.Layer.Features {
		for _, v := range f.Vulnerabilities {

			result[types.Priority(v.Severity)] = append(result[types.Priority(v.Severity)], vulnerabilityWithFeature{Vulnerability: v, Feature: f.Name + ":" + f.Version})
		}
	}

	return result
}

// SortedVulnerabilities get all vulnerabilities sorted by Severity
func sortedVulnerabilities(imageAnalysis ImageAnalysis) []v1.Feature {
	features := []v1.Feature{}

	l := imageAnalysis.MostRecentLayer()

	for _, f := range l.Layer.Features {
		if len(f.Vulnerabilities) > 0 {
			vulnerabilities := []v1.Vulnerability{}
			for _, p := range invertedPriorities() {
				for _, v := range f.Vulnerabilities {
					if types.Priority(v.Severity) == p {
						vulnerabilities = append(vulnerabilities, v)
					}
				}
			}
			nf := f
			nf.Vulnerabilities = vulnerabilities
			features = append(features, nf)
		}
	}

	return features
}

func SaveReport(name string, content string) error {
	path := viper.GetString("clair.report.path") + "/" + Report.Format
	if err := os.MkdirAll(path, 0777); err != nil {
		return err
	}

	reportsName := fmt.Sprintf("%v/analysis-%v.%v", path, name, strings.ToLower(Report.Format))
	f, err := os.Create(reportsName)
	if err != nil {
		return fmt.Errorf("creating report file: %v", err)
	}

	_, err = f.WriteString(content)

	if err != nil {
		return fmt.Errorf("writing report file: %v", err)
	}
	fmt.Printf("%v report at %v\n", strings.ToUpper(Report.Format), reportsName)
	return nil
}

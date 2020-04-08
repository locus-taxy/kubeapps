/*
Copyright (c) 2018 The Helm Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/kubeapps/common/response"
	"github.com/kubeapps/kubeapps/pkg/chart/models"
	log "github.com/sirupsen/logrus"
)

// Params a key-value map of path params
type Params map[string]string

// WithParams can be used to wrap handlers to take an extra arg for path params
type WithParams func(http.ResponseWriter, *http.Request, Params)

func (h WithParams) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	h(w, req, vars)
}

const chartCollection = "charts"
const filesCollection = "files"

type apiResponse struct {
	ID            string      `json:"id"`
	Type          string      `json:"type"`
	Attributes    interface{} `json:"attributes"`
	Links         interface{} `json:"links"`
	Relationships relMap      `json:"relationships"`
}

type apiListResponse []*apiResponse

type selfLink struct {
	Self string `json:"self"`
}

type relMap map[string]rel
type rel struct {
	Data  interface{} `json:"data"`
	Links selfLink    `json:"links"`
}

type meta struct {
	TotalPages int `json:"totalPages"`
}

// count is used to parse the result of a $count operation in the database
type count struct {
	Count int
}

// getPageNumberAndSize extracts the page number and size of a request. Default (1, 0) if not set
func getPageNumberAndSize(req *http.Request) (int, int) {
	page := req.FormValue("page")
	size := req.FormValue("size")
	pageInt, err := strconv.ParseUint(page, 10, 64)
	if err != nil {
		pageInt = 1
	}
	// ParseUint will return 0 if size is a not positive integer
	sizeInt, _ := strconv.ParseUint(size, 10, 64)
	return int(pageInt), int(sizeInt)
}

// showDuplicates returns if a request wants to retrieve charts. Default false
func showDuplicates(req *http.Request) bool {
	return len(req.FormValue("showDuplicates")) > 0
}

// min returns the minimum of two integers.
// We are not using math.Min since that compares float64
// and it's unnecessarily complex.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func uniqChartList(charts []*models.Chart) []*models.Chart {
	// We will keep track of unique digest:chart to avoid duplicates
	chartDigests := map[string]bool{}
	res := []*models.Chart{}
	for _, c := range charts {
		digest := c.ChartVersions[0].Digest
		// Filter out the chart if we've seen the same digest before
		if _, ok := chartDigests[digest]; !ok {
			chartDigests[digest] = true
			res = append(res, c)
		}
	}
	return res
}

func getPaginatedChartList(repo string, pageNumber, pageSize int, showDuplicates bool) (apiListResponse, interface{}, error) {
	charts, totalPages, err := manager.getPaginatedChartList(repo, pageNumber, pageSize, showDuplicates)
	return newChartListResponse(charts), meta{totalPages}, err
}

// listCharts returns a list of charts
func listCharts(w http.ResponseWriter, req *http.Request) {
	pageNumber, pageSize := getPageNumberAndSize(req)
	cl, meta, err := getPaginatedChartList("", pageNumber, pageSize, showDuplicates(req))
	if err != nil {
		log.WithError(err).Error("could not fetch charts")
		response.NewErrorResponse(http.StatusInternalServerError, "could not fetch all charts").Write(w)
		return
	}
	response.NewDataResponseWithMeta(cl, meta).Write(w)
}

// listRepoCharts returns a list of charts in the given repo
func listRepoCharts(w http.ResponseWriter, req *http.Request, params Params) {
	pageNumber, pageSize := getPageNumberAndSize(req)
	cl, meta, err := getPaginatedChartList(params["repo"], pageNumber, pageSize, showDuplicates(req))
	if err != nil {
		log.WithError(err).Error("could not fetch charts")
		response.NewErrorResponse(http.StatusInternalServerError, "could not fetch all charts").Write(w)
		return
	}
	response.NewDataResponseWithMeta(cl, meta).Write(w)
}

// getChart returns the chart from the given repo
func getChart(w http.ResponseWriter, req *http.Request, params Params) {
	chartID := fmt.Sprintf("%s/%s", params["repo"], params["chartName"])
	chart, err := manager.getChart(chartID)
	if err != nil {
		log.WithError(err).Errorf("could not find chart with id %s", chartID)
		response.NewErrorResponse(http.StatusNotFound, "could not find chart").Write(w)
		return
	}

	cr := newChartResponse(&chart)
	response.NewDataResponse(cr).Write(w)
}

// listChartVersions returns a list of chart versions for the given chart
func listChartVersions(w http.ResponseWriter, req *http.Request, params Params) {
	chartID := fmt.Sprintf("%s/%s", params["repo"], params["chartName"])
	chart, err := manager.getChart(chartID)
	if err != nil {
		log.WithError(err).Errorf("could not find chart with id %s", chartID)
		response.NewErrorResponse(http.StatusNotFound, "could not find chart").Write(w)
		return
	}

	cvl := newChartVersionListResponse(&chart)
	response.NewDataResponse(cvl).Write(w)
}

// getChartVersion returns the given chart version
func getChartVersion(w http.ResponseWriter, req *http.Request, params Params) {
	chartID := fmt.Sprintf("%s/%s", params["repo"], params["chartName"])
	chart, err := manager.getChartVersion(chartID, params["version"])
	if err != nil {
		log.WithError(err).Errorf("could not find chart with id %s", chartID)
		response.NewErrorResponse(http.StatusNotFound, "could not find chart version").Write(w)
		return
	}

	cvr := newChartVersionResponse(&chart, chart.ChartVersions[0])
	response.NewDataResponse(cvr).Write(w)
}

// getChartIcon returns the icon for a given chart
func getChartIcon(w http.ResponseWriter, req *http.Request, params Params) {
	chartID := fmt.Sprintf("%s/%s", params["repo"], params["chartName"])
	chart, err := manager.getChart(chartID)
	if err != nil {
		log.WithError(err).Errorf("could not find chart with id %s", chartID)
		http.NotFound(w, req)
		return
	}

	if chart.RawIcon == nil {
		http.NotFound(w, req)
		return
	}

	if chart.IconContentType != "" {
		// Force the Content-Type header because the autogenerated type does not work for
		// image/svg+xml. It is detected as plain text
		w.Header().Set("Content-Type", chart.IconContentType)
	}

	w.Write(chart.RawIcon)
}

// getChartVersionReadme returns the README for a given chart
func getChartVersionReadme(w http.ResponseWriter, req *http.Request, params Params) {
	fileID := fmt.Sprintf("%s/%s-%s", params["repo"], params["chartName"], params["version"])
	files, err := manager.getChartFiles(fileID)
	if err != nil {
		log.WithError(err).Errorf("could not find files with id %s", fileID)
		http.NotFound(w, req)
		return
	}
	readme := []byte(files.Readme)
	if len(readme) == 0 {
		log.Errorf("could not find a README for id %s", fileID)
		http.NotFound(w, req)
		return
	}
	w.Write(readme)
}

// getChartVersionValues returns the values.yaml for a given chart
func getChartVersionValues(w http.ResponseWriter, req *http.Request, params Params) {
	fileID := fmt.Sprintf("%s/%s-%s", params["repo"], params["chartName"], params["version"])
	files, err := manager.getChartFiles(fileID)
	if err != nil {
		log.WithError(err).Errorf("could not find values.yaml with id %s", fileID)
		http.NotFound(w, req)
		return
	}

	w.Write([]byte(files.Values))
}

// getChartVersionSchema returns the values.schema.json for a given chart
func getChartVersionSchema(w http.ResponseWriter, req *http.Request, params Params) {
	fileID := fmt.Sprintf("%s/%s-%s", params["repo"], params["chartName"], params["version"])
	files, err := manager.getChartFiles(fileID)
	if err != nil {
		log.WithError(err).Errorf("could not find values.schema.json with id %s", fileID)
		http.NotFound(w, req)
		return
	}

	w.Write([]byte(files.Schema))
}

// listChartsWithFilters returns the list of repos that contains the given chart and the latest version found
func listChartsWithFilters(w http.ResponseWriter, req *http.Request, params Params) {
	charts, err := manager.getChartsWithFilters(params["chartName"], req.FormValue("version"), req.FormValue("appversion"))
	if err != nil {
		log.WithError(err).Errorf(
			"could not find charts with the given name %s, version %s and appversion %s",
			params["chartName"], req.FormValue("version"), req.FormValue("appversion"),
		)
		// continue to return empty list
	}

	chartResponse := charts
	if !showDuplicates(req) {
		chartResponse = uniqChartList(charts)
	}
	cl := newChartListResponse(chartResponse)
	response.NewDataResponse(cl).Write(w)
}

// searchCharts returns the list of charts that matches the query param in any of these fields:
//  - name
//  - description
//  - repository name
//  - any keyword
//  - any source
//  - any maintainer name
func searchCharts(w http.ResponseWriter, req *http.Request, params Params) {
	query := req.FormValue("q")
	repo := params["repo"]
	charts, err := manager.searchCharts(query, repo)
	if err != nil {
		log.WithError(err).Errorf(
			"could not find charts with the given query %s",
			query,
		)
		// continue to return empty list
	}

	chartResponse := charts
	if !showDuplicates(req) {
		chartResponse = uniqChartList(charts)
	}
	cl := newChartListResponse(chartResponse)
	response.NewDataResponse(cl).Write(w)
}

func newChartResponse(c *models.Chart) *apiResponse {
	latestCV := c.ChartVersions[0]
	return &apiResponse{
		Type:       "chart",
		ID:         c.ID,
		Attributes: blankRawIconAndChartVersions(chartAttributes(*c)),
		Links:      selfLink{pathPrefix + "/charts/" + c.ID},
		Relationships: relMap{
			"latestChartVersion": rel{
				Data:  chartVersionAttributes(c.ID, latestCV, c.Description),
				Links: selfLink{pathPrefix + "/charts/" + c.ID + "/versions/" + latestCV.Version},
			},
		},
	}
}

// blankRawIconAndChartVersions returns the same chart data but with a blank raw icon field and no chartversions.
// TODO(mnelson): The raw icon data should be stored in a separate postgresql column
// rather than the json field so that this isn't necessary.
func blankRawIconAndChartVersions(c models.Chart) models.Chart {
	c.RawIcon = nil
	c.ChartVersions = []models.ChartVersion{}
	return c
}

func newChartListResponse(charts []*models.Chart) apiListResponse {
	cl := apiListResponse{}
	for _, c := range charts {
		cl = append(cl, newChartResponse(c))
	}
	return cl
}

func chartVersionAttributes(cid string, cv models.ChartVersion, description string) models.ChartVersion {
	cv.Readme = pathPrefix + "/assets/" + cid + "/versions/" + cv.Version + "/README.md"
	cv.Values = pathPrefix + "/assets/" + cid + "/versions/" + cv.Version + "/values.yaml"
	cv.Description = description
	return cv
}

func chartAttributes(c models.Chart) models.Chart {
	if c.RawIcon != nil {
		c.Icon = pathPrefix + "/assets/" + c.ID + "/logo"
	} else {
		// If the icon wasn't processed, it is either not set or invalid
		c.Icon = ""
	}
	return c
}

func newChartVersionResponse(c *models.Chart, cv models.ChartVersion) *apiResponse {
	return &apiResponse{
		Type:       "chartVersion",
		ID:         fmt.Sprintf("%s-%s", c.ID, cv.Version),
		Attributes: chartVersionAttributes(c.ID, cv, c.Description),
		Links:      selfLink{pathPrefix + "/charts/" + c.ID + "/versions/" + cv.Version},
		Relationships: relMap{
			"chart": rel{
				Data:  blankRawIconAndChartVersions(chartAttributes(*c)),
				Links: selfLink{pathPrefix + "/charts/" + c.ID},
			},
		},
	}
}

func newChartVersionListResponse(c *models.Chart) apiListResponse {
	var cvl apiListResponse
	for _, cv := range c.ChartVersions {
		cvl = append(cvl, newChartVersionResponse(c, cv))
	}

	return cvl
}

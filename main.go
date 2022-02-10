package main

import (
	"encoding/json"
	"fmt"
	"os"
	"math"
	"io/ioutil"
	"github.com/paulmach/go.geojson"
	"github.com/jszwec/csvutil"
    "gonum.org/v1/gonum/graph/simple"
    "gonum.org/v1/gonum/graph/path"
)

type City struct {
	Size		int		`csv:"size"`
	Name		string	`csv:"name"`
	Longitude	float64	`csv:"longitude"`
	Latitude	float64	`csv:"latitude"`
}


// The attributes here are slices of slices of floats.
// So just the coordinates and thats all we need.
type networkGeoData struct {
	road 	[][]float64
	rail 	[][]float64
	sea 	[][]float64
	cities	[]City
}


func readFileBytes(filename string) []byte {
	// Open file
	fileIO, err1 := os.Open(filename)
	if err1 != nil {
		fmt.Println(err)
	}
	defer fileIO.Close()
	// Read bytes
	raw, err2 := ioutil.ReadAll(fileIO)
	if err1 != nil {
		fmt.Println(err)
	}
	return raw
}


func readCities(filename string) []City {
	rawCSV := readFileBytes(filename)
    var cities []City
    if err := csvutil.Unmarshal(rawCSV, cities); err != nil {
        fmt.Println("error:", err)
    }
    return cities
}


func geoJsonToData(filename string) [][]float64 {
	rawJSON := readFileBytes(filename)
	
	// Unmarshal geojson
	featureCollection := geojson.NewFeatureCollection()
	_ = json.Unmarshal(rawJSON, featureCollection)

	// Put features into our simple data format
	var result [][]float64
	for i := 0; i < len(featureCollection.Features); i++ {
		
		feat := featureCollection.Features[i]
		
		if feat.Geometry.Type == "MultiLineString" {
			for j := 0; j < len(feat.Geometry.MultiLineString); j++ {
				ls := feat.Geometry.MultiLineString[j]
				result = append(result, ls...)
			}
		} else if feat.Geometry.Type == "LineString" {
			ls := feat.Geometry.LineString
			result = append(result, ls...)
		}
	}

	return result
}


func getGeoData(roadFileName string, railFileName string, seaFileName string, cityFileName string) networkGeoData {
	var roadData [][]float64 = geoJsonToData(roadFileName)
	var railData [][]float64 = geoJsonToData(railFileName)
	var seaData [][]float64 = geoJsonToData(seaFileName)
	var cityData []City = readCities(cityFileName)

	result := networkGeoData{
		road: roadData,
		rail: railData,
		sea: seaData,
		cities: cityData,
	}

	return result
}


func initializeGraph() {
	// self - weight of a self-connection
	// abset - weight of an absent connection
	var self int = 0
	var absent int = math.Inf(1)
	// We use a directed graph because transport costs are not necessarily symmetrical
	g := simple.NewWeightedDirectedGraph(self, absent)
	return g
}


func addTransportEdges(g, myGeoData) {
	
}


func matchCitiesToNodes(cities []City, nodes []) map[int]int {
	var result map[int]int

}


func main() {
	// 1. Read geojsons
	var roadFileName string = "/Users/work/Documents/bri_market_access/data/geojson/roads_prebri.geojson"
	var railFileName string = "/Users/work/Documents/bri_market_access/data/geojson/rails_prebri.geojson"
	var seaFileName string = "/Users/work/Documents/bri_market_access/data/geojson/sea_prebri.geojson"
	myGeoData := getGeoData(roadFileName, railFileName, seaFileName)
	// fmt.Println(myGeoData)

	// 2. Create graph
	g := initializeGraph()
	g := addTransportEdges(g, myGeoData)


	//	2a. Match cities to nodes
	cityNodes := matchCitiesToNodes(myGeoData)
	//	2b. Add segment costs to graph
	//	2c. Create border crossings
	//	2d. Create intermodal transfers
	//	2e. Add external market links

	// 3. Create cost matrix
}
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
    "github.com/asmarques/geodist"
)


// Point represents a geographic 2D coordinate
type Point struct {
	Lon float64
	Lat float64
}


// https://github.com/olivermichel/vincenty/blob/master/geo.go
// Vincenty calculates the great circle distance between Points x and y
// The distance is returned in meters, x and y must be specified in degrees
func Vincenty(x, y Point) float64 {

	sq := func(x float64) float64 { return x * x }
	degToRad := func(x float64) float64 { return x * math.Pi / 180 }

	var lambda, tmp, q, p float64 = 0, 0, 0, 0
	var sigma, sinSigma, cosSigma float64 = 0, 0, 0
	var sinAlpha, cos2Alpha, cos2Sigma float64 = 0, 0, 0
	var c float64 = 0

	A := 6378137.0
	F := 1 / 298.257223563
	B := (1 - F) * A
	C := (sq(A) - sq(B)) / sq(B)

	uX := math.Atan((1 - F) * math.Tan(degToRad(x.Lat)))
	sinUX := math.Sin(uX)
	cosUX := math.Cos(uX)

	uY := math.Atan((1 - F) * math.Tan(degToRad(y.Lat)))
	sinUY := math.Sin(uY)
	cosUY := math.Cos(uY)

	l := degToRad(y.Lon) - degToRad(x.Lon)

	lambda = l

	for i := 0; i < 10; i++ {

		tmp = math.Cos(lambda)
		q = cosUY * math.Sin(lambda)
		p = cosUX*sinUY - sinUX*cosUY*tmp

		sinSigma = math.Sqrt(q*q + p*p)
		cosSigma = sinUX*sinUY + cosUX*cosUY*tmp
		sigma = math.Atan2(sinSigma, cosSigma)

		sinAlpha = (cosUX * cosUY * math.Sin(lambda)) / sinSigma
		cos2Alpha = 1 - sq(sinAlpha)
		cos2Sigma = cosSigma - (2*sinUX*sinUY)/cos2Alpha

		c = F / 16.0 * cos2Alpha * (4 + F*(4-3*cos2Alpha))
		tmp = lambda
		lambda = (l + (1-c)*F*sinAlpha*(sigma+c*sinSigma*(cos2Sigma+c*cosSigma*(-1+2*cos2Sigma*cos2Sigma))))

		if math.Abs(lambda-tmp) < 0.00000001 {
			break
		}
	}

	uu := cos2Alpha * C
	a := 1 + uu/16384*(4096+uu*(-768+uu*(320-175*uu)))
	b := uu / 1024 * (256 + uu*(-128+uu*(74-47*uu)))

	deltaSigma := (b * sinSigma * (cos2Sigma + 1.0/4.0*b*(cosSigma*(-1+2*sq(cos2Sigma))*(-3+4*sq(sinSigma))*(-3+4*sq(cos2Sigma)))))

	return B * a * (sigma - deltaSigma)
}


type City struct {
	Size		int		`csv:"size"`
	Name		string	`csv:"name"`
	Longitude	float64	`csv:"longitude"`
	Latitude	float64	`csv:"latitude"`
}


// The attributes here are slices of slices of floats.
// So just the coordinates and thats all we need.
type networkGeoData struct {
	road 	[][]Point
	rail 	[][]Point
	sea 	[][]Point
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


func lineStringToPointArr(ls [][]float64) []Point {
	var result []Point
	for i := 0; i < len(ls); i++ {
		var pt Point = Point{ls[i][0], ls[i][1]}
		result = append(result, pt)
	}
	return result
}


func geoJsonToData(filename string) [][]Point {
	rawJSON := readFileBytes(filename)
	
	// Unmarshal geojson
	featureCollection := geojson.NewFeatureCollection()
	_ = json.Unmarshal(rawJSON, featureCollection)

	// Put features into our simple data format
	var result [][]Point
	for i := 0; i < len(featureCollection.Features); i++ {
		
		feat := featureCollection.Features[i]
		
		if feat.Geometry.Type == "MultiLineString" {
			for j := 0; j < len(feat.Geometry.MultiLineString); j++ {
				ls := feat.Geometry.MultiLineString[j]
				pointArr := lineStringToPointArr(ls)
				result = append(result, pointArr)
			}
		} else if feat.Geometry.Type == "LineString" {
			ls := feat.Geometry.LineString
			pointArr := lineStringToPointArr(ls)
			result = append(result, pointArr)
		}
	}

	return result
}


func getGeoData(roadFileName string, railFileName string, seaFileName string, cityFileName string) networkGeoData {
	var roadData [][][]float64 = geoJsonToData(roadFileName)
	var railData [][][]float64 = geoJsonToData(railFileName)
	var seaData [][][]float64 = geoJsonToData(seaFileName)
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


func getLength(linestring [][]float64) float64 {

}


func addTransportEdges(g simple.NewWeightedDirectedGraph, myGeoData networkGeoData) simple.NewWeightedDirectedGraph {
	var nodeIdMap map[[2]int]int //[lon, lat] -> id

	// roads
	for i := 0; i < len(myGeoData.road); i++ {
		ls := myGeoData.road[i]
		length := getLength(ls)
		// Start at first
   		for j := 1; j < len(ls); j++ {
   			// Both directions
   			id_from := ls[j-1]
   			id_to := ls[j]
   			weightedEdge := simple.WeightedEdge{F: simple.Node(id_from), T: simple.Node(id_to), W: weight}
   		}

	}

    graph.SetWeightedEdge(weightedEdge)
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
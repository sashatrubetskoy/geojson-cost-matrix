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
	// "gonum.org/v1/gonum/graph/path"
)


// Point represents a geographic 2D coordinate
type Point struct {
	Lon float64
	Lat float64
}


// https://github.com/olivermichel/vincenty/blob/master/geo.go
// Vincenty calculates the great circle distance between Points x and y
// The distance is returned in meters, x and y must be specified in degrees
func vincenty(x, y Point) float64 {

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
		fmt.Println(err1)
	}
	defer fileIO.Close()
	// Read bytes
	raw, err2 := ioutil.ReadAll(fileIO)
	if err1 != nil {
		fmt.Println(err2)
	}
	return raw
}


func readCities(filename string) []City {
	rawCSV := readFileBytes(filename)
	var cities []City
	if err := csvutil.Unmarshal(rawCSV, &cities); err != nil {
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
	var roadData [][]Point = geoJsonToData(roadFileName)
	var railData [][]Point = geoJsonToData(railFileName)
	var seaData [][]Point = geoJsonToData(seaFileName)
	var cityData []City = readCities(cityFileName)

	result := networkGeoData{
		road: roadData,
		rail: railData,
		sea: seaData,
		cities: cityData,
	}

	return result
}


func initializeGraph() *simple.WeightedDirectedGraph {
	// self - weight of a self-connection
	// abset - weight of an absent connection
	var self float64 = 0
	var absent float64 = math.Inf(1)
	// We use a directed graph because transport costs are not necessarily symmetrical
	g := simple.NewWeightedDirectedGraph(self, absent)
	return g
}


func getLength(linestring []Point) float64 {
	var totalLength float64 = 0
	for i := 1; i < len(linestring); i++ {
		dist := vincenty(linestring[i-1], linestring[i])
		totalLength = totalLength + dist
	}
	return totalLength
}


func addTransportEdges(g *simple.WeightedDirectedGraph, geoData [][]Point) *simple.WeightedDirectedGraph {
	var lastID int = 0
	pointToNodeIdx := make(map[Point]int)

	// roads
	for i := 0; i < len(geoData); i++ {
		linestring := geoData[i]
		for j := 1; j < len(linestring); j++ {
			cur := linestring[j]
			prev := linestring[j-1]

			length := vincenty(prev, cur)
			
			id_from, ok := pointToNodeIdx[cur]
			if !ok {
				lastID++
				pointToNodeIdx[cur] = lastID
			}

			id_to, ok := pointToNodeIdx[prev]
			if !ok {
				lastID++
				pointToNodeIdx[prev] = lastID
			}

			// Both directions
			if id_to != id_from {
				weightedEdge1 := simple.WeightedEdge{F: simple.Node(id_from), T: simple.Node(id_to), W: length}
				g.SetWeightedEdge(weightedEdge1)
				weightedEdge2 := simple.WeightedEdge{F: simple.Node(id_to), T: simple.Node(id_from), W: length}
				g.SetWeightedEdge(weightedEdge2)
			}
		}
	}

	return g
}


// func matchCitiesToNodes(cities []City, nodes []) map[int]int {
// 	var result map[int]int
// 	return
// }


func main() {
	// 1. Read geojsons
	var roadFileName string = "/Users/work/Documents/bri_market_access/data/geojson/roads_prebri.geojson"
	var railFileName string = "/Users/work/Documents/bri_market_access/data/geojson/rails_prebri.geojson"
	var seaFileName string = "/Users/work/Documents/bri_market_access/data/geojson/sea_prebri.geojson"
	var cityFileName string = "/Users/work/Documents/bri_market_access/data/csv/cities.csv"
	myGeoData := getGeoData(roadFileName, railFileName, seaFileName, cityFileName)
	// fmt.Println(myGeoData)

	// 2. Create graph
	g := initializeGraph()
	g = addTransportEdges(g, myGeoData.road)
	g = addTransportEdges(g, myGeoData.rail)
	g = addTransportEdges(g, myGeoData.sea)


	//	2a. Match cities to nodes
	// cityNodes := matchCitiesToNodes(myGeoData)
	//	2b. Add segment costs to graph
	//	2c. Create border crossings
	//	2d. Create intermodal transfers
	//	2e. Add external market links

	// 3. Create cost matrix
}
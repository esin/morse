package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"strings"

	b64 "encoding/base64"

	wav "github.com/youpy/go-wav"
	yaml "gopkg.in/yaml.v3"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type codesStruct struct {
	Letters map[string]Letter
}

type Letter struct {
	Code []string
}

var data codesStruct
var symbolsLookup map[string]string

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log.Println("Reading codes.yaml")
	codes, err := ioutil.ReadFile("codes.yaml")
	if err != nil {
		log.Fatal(err)
	}

	err = yaml.Unmarshal(codes, &data)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Done reading codes.yaml")

	// Развернём в другую сторону таблицу соответствия символов и кодов
	symbolsLookup = make(map[string]string)
	for k, v := range data.Letters {
		for _, i := range v.Code {
			symbolsLookup[i] = k
		}
	}
}

func codeSoundGen(inText string) bytes.Buffer {
	// За единицу времени принимается длительность одной точки. Длительность тире равна трём точкам
	// Пауза между элементами одного знака — одна точка, между знаками в слове — 3 точки, пробел — 7 точек

	var numSamples uint32 = 0
	var countSamples uint32 = 0
	finalSample := make([]wav.Sample, 0)

	inText = strings.ReplaceAll(inText, "       ", " / ") // чтобы находить 7 пробелов
	inText = strings.ReplaceAll(inText, "   ", " ")       // чтобы находить тройные пробелы

	for _, ch := range inText {
		samples := make([]wav.Sample, 0)

		switch string(ch) {
		case ".":
			{
				numSamples = 200
				for i := 0; uint32(i) < numSamples; i++ {
					// samples[i].Values[0] = int(32767.0 * math.Sin(2.0*math.Pi*800.0/2000.0*float64(i)))
					samples = append(samples, wav.Sample{
						Values: [2]int{int(32767.0 * math.Sin(2.0*math.Pi*800.0/2000.0*float64(i))), 0},
					})
				}
				for i := 0; uint32(i) < 200; i++ {
					samples = append(samples, wav.Sample{
						Values: [2]int{0, 0},
					})
				}
				countSamples += 200 + numSamples
			}
		case "-":
			{
				numSamples = 600
				for i := 0; uint32(i) < numSamples; i++ {
					samples = append(samples, wav.Sample{
						Values: [2]int{int(32767.0 * math.Sin(2.0*math.Pi*800.0/2000.0*float64(i))), 0},
					})
				}
				for i := 0; uint32(i) < 200; i++ {
					samples = append(samples, wav.Sample{
						Values: [2]int{0, 0},
					})
				}
				countSamples += 200 + numSamples
			}

		case " ":
			{
				numSamples = 400 // 200 + 400
				for i := 0; uint32(i) < 400; i++ {
					samples = append(samples, wav.Sample{
						Values: [2]int{0, 0},
					})
				}
				countSamples += 200 + numSamples
			}
		case "/":
			{
				numSamples = 1000 // 200 + 1000 + 200 = 200 * 7
				for i := 0; uint32(i) < 1000; i++ {
					samples = append(samples, wav.Sample{
						Values: [2]int{0, 0},
					})
				}
				countSamples += 200 + numSamples
			}

		}

		finalSample = append(finalSample, samples...)
	}

	buf := new(bytes.Buffer)
	writer := wav.NewWriter(buf, countSamples, 1, 2000, 16)
	err := writer.WriteSamples(finalSample)
	if err != nil {
		// TODO: add err
		fmt.Println(err)
	}

	return *buf
}

func encode(inText string) string {
	log.Println("Incoming text:", inText)
	finishText := ""
	// Ух ты, прожевали. Погнали дальше
	for _, ch := range strings.ToUpper(inText) {
		code, ok := data.Letters[string(ch)]
		if ok {
			if len(code.Code) > 0 {
				finishText += code.Code[0] + " "
			}
		}
	}
	// Удалить последние пробелы
	if len(finishText) > 1 {
		finishText = finishText[:len(finishText)-1]
	}

	log.Println("Finish text:", finishText)
	return finishText
}

func decode(inText string) string {
	log.Println("Incoming text:", inText)

	finishText := ""

	// Разбиваем на слова
	words := strings.Split(inText, "       ")

	for _, word := range words {
		log.Println("WORD: ", word)
		// Разбиваем на буквы
		letters := strings.Split(word, " ")
		for _, l := range letters {
			// Ищем символ
			if len(symbolsLookup[l]) > 0 {
				finishText = finishText + symbolsLookup[l]
			}
		}
		finishText = finishText + " "
	}

	log.Println("Finish text:", finishText)
	return finishText
}

func lambdaHandler(event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	replyHeaders := make(map[string]string, 0)
	replyHeaders["Content-Type"] = "text/plain"
	replyHeaders["Access-Control-Allow-Origin"] = "*"

	incomingText := ""
	if len(event.QueryStringParameters["text"]) <= 0 {
		return events.APIGatewayProxyResponse{Body: "ERR: text not found", StatusCode: 400, Headers: replyHeaders}, nil
	}

	incomingText = event.QueryStringParameters["text"]

	if event.Path == "/encode" {
		finishText := encode(incomingText)
		return events.APIGatewayProxyResponse{Body: finishText, StatusCode: 200, Headers: replyHeaders}, nil
	}

	if event.Path == "/decode" {
		finishText := decode(incomingText)
		return events.APIGatewayProxyResponse{Body: finishText, StatusCode: 200, Headers: replyHeaders}, nil
	}

	if event.Path == "/encodesound" {
		finishText := encode(incomingText)
		l := codeSoundGen(finishText)
		sEnc := b64.StdEncoding.EncodeToString(l.Bytes())
		replyHeaders["Content-Type"] = "audio/x-wav"
		replyHeaders["Content-Disposition"] = fmt.Sprintf("attachment; filename=\"%s.wav\"", incomingText)

		return events.APIGatewayProxyResponse{Body: sEnc, StatusCode: 200, Headers: replyHeaders, IsBase64Encoded: true}, nil
	}

	return events.APIGatewayProxyResponse{Body: "ERR", StatusCode: 404}, nil
}

func main() {
	lambda.Start(lambdaHandler)
}

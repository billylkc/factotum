package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// ReqHead as request header
type ReqHead struct {
	URL     string `json:"url"`
	Method  string `json:"method"`
	Headers struct {
		Referer        string `json:"Referer"`
		XRequestedWith string `json:"X-Requested-With"`
		UserAgent      string `json:"User-Agent"`
		Accept         string `json:"Accept"`
	} `json:"headers"`
	MixedContentType string `json:"mixedContentType"`
	InitialPriority  string `json:"initialPriority"`
	ReferrerPolicy   string `json:"referrerPolicy"`
}

func main() {

	// Flag
	var (
		url      string
		timeout  int
		verbose  bool
		jsonOnly bool
	)

	// Prompt user to input url if non given
	flag.StringVar(&url, "url", "", "Some URL, e.g. https://www.mannings.com.hk")
	flag.IntVar(&timeout, "timeout", 15, "Timeout for network listening, e.g. 15")
	flag.BoolVar(&verbose, "verbose", false, "Show extended result or not")
	flag.BoolVar(&jsonOnly, "jsonOnly", true, "Save JSON Results only")
	flag.Parse()

	if url == "" {
		flag.PrintDefaults()
		fmt.Println("")
		fmt.Println("------------------------------------------------------------------------")
		fmt.Println("Example: ")
		fmt.Println(`      ./cmd/Factotum.exe -url="https://www.mannings.com.hk" -timeout=15 -verbose=false -jsonOnly=true`)
		fmt.Println("")
		fmt.Println("Example: ")
		fmt.Println(`      go run main.go -url="https://www.mannings.com.hk" -timeout=15 -verbose=false -jsonOnly=true`)
		fmt.Println("------------------------------------------------------------------------")
		fmt.Println("")
		os.Exit(1)
	}

	// Set minimum timeout
	if timeout < 5 {
		timeout = 5
	}

	PrintBlock("Parameters", "*")

	fmt.Printf("  url - %s \n", url)
	fmt.Printf("  timeout - %d \n", timeout)
	fmt.Printf("  verbose - %t \n", verbose)
	fmt.Printf("  jsonOnly - %t \n", jsonOnly)

	// Define url
	// url := "https://hk.centanet.com/estate/%E5%A4%AA%E5%8F%A4%E5%9F%8E/3-OVDUURFSRJ"
	resultFolder, _ := getResultFolder(url)
	fmt.Println("")
	fmt.Printf("Writing to Output Folder - %s \n", resultFolder)
	fmt.Println("")

	time.Sleep(3 * time.Second)
	fmt.Println("Start Getting Content")

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", true),
		chromedp.Flag("ignore-cerificate-errors", true),
		// chromedp.UserDataDir(dir),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	taskCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	taskCtx, cancel = context.WithTimeout(taskCtx, time.Duration(timeout)*time.Second)
	defer cancel()

	if err := chromedp.Run(taskCtx); err != nil {
		panic(err)
	}

	anotherListenFunc(taskCtx, resultFolder, verbose, jsonOnly)

	chromedp.Run(taskCtx,
		network.Enable(),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.BySearch),
	)

	if !jsonOnly {
		select {
		case <-time.After(time.Duration(timeout) * time.Second):
			// Print Get program
			fmt.Println("")
			fmt.Println("Post processing - Writing Get reqeust to Go files")
			fmt.Println("")

			files, err := ioutil.ReadDir(resultFolder)
			if err != nil {
				log.Fatal(err)
			}

			for _, file := range files {
				if strings.Contains(file.Name(), "GET") {
					f := path.Join(resultFolder, file.Name())
					PrintGetReq(f)

				}
			}

			fmt.Printf("Save to %s\\main.go.xxx \n", resultFolder)
		}
	}
}

func anotherListenFunc(ctx context.Context, p string, verbose bool, jsonOnly bool) {
	chromedp.ListenTarget(
		ctx,
		func(ev interface{}) {
			if ev, ok := ev.(*network.EventResponseReceived); ok {

				if ev.Type != "XHR" {
					return
				}

				if verbose {
					fmt.Println(ev.Response)
					fmt.Println("event received:")
					fmt.Println(ev.Type)
				}

				// Check Response header is JSON for both stupid Content-Type and content-type
				contentType := ev.Response.Headers["Content-Type"]
				if contentType == nil {
					contentType = ev.Response.Headers["content-type"]
				}
				abc, ok := contentType.(string) // Parse interface to string
				if !ok {
					return
				}

				if !strings.Contains(abc, "application/json") {
					return
				}

				go func() {
					// print response body
					c := chromedp.FromContext(ctx)
					rbp := network.GetResponseBody(ev.RequestID)
					body, err := rbp.Do(cdp.WithExecutor(ctx, c.Target))
					if err != nil {
						fmt.Println(err)
					}
					if string(body) != "{}" {

						filename := filepath.Join(p, "RESULT-"+ev.RequestID.String())
						if err = ioutil.WriteFile(filename, body, 0644); err != nil {
							log.Fatal(err)
						}
						if err == nil {
							// fmt.Printf("%s\n", body)
						}
					}

				}()

			}
			if ev, ok := ev.(*network.EventRequestWillBeSent); ok {
				if jsonOnly == false {
					if ev.Type != "XHR" {
						return
					}

					if !jsonOnly {

						r := ev.Request
						method := r.Method
						filename := filepath.Join(p, method+"-"+ev.RequestID.String())
						req := PrettyPrint(ev.Request)

						if err := ioutil.WriteFile(filename, []byte(req), 0644); err != nil {
							log.Fatal(err)
						}
					}
				}
			}
		})
}

// getResultFolder create the result folder if not exists
// Return the relative path
func getResultFolder(s string) (absPath string, err error) {
	var (
		host      string
		path      string
		firstPart string
	)

	u, err := url.Parse(s)
	host = u.Host
	path = u.Path

	temp := strings.Split(path, "/")
	if len(temp) > 1 {
		firstPart = temp[1]
	}

	if firstPart != "" {
		absPath = fmt.Sprintf("%s_%s", host, firstPart)
	} else {
		absPath = host
	}

	// Derive path, replace special characters, add output folder
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	absPath = reg.ReplaceAllString(absPath, "_")
	absPath, _ = filepath.Abs(filepath.Join("output", absPath))

	// Create folder
	err = os.MkdirAll(absPath, 0755)
	if err != nil {
		panic(err)
	}

	return absPath, err
}

// PrintGetReq prints Get Program in differernt languages (e.g. Go, Python)
func PrintGetReq(path string) {

	file, _ := ioutil.ReadFile(path)
	resultGoFile := strings.ReplaceAll(path, "GET-", "main.go.")
	resultPyFile := strings.ReplaceAll(path, "GET-", "main.py.")
	data := ReqHead{}
	_ = json.Unmarshal([]byte(file), &data)

	// Define template - Python
	const pyTemplate = `import requests

def main():

	url = "{{.URL}}"
	
	payload  = {}
	headers = {
	'Accept': '{{.Headers.Accept}}',
	'Referer': '{{.Headers.Referer}}',
	'User-Agent': '{{.Headers.UserAgent}}'
	}
	
	response = requests.request("{{.Method}}", url, headers=headers, data = payload)
	
	print(response.text.encode('utf8'))	

if __name__ == "__main__":
	main()
`

	// Define template - Go
	const goTemplate = `package main

import (
	"fmt"
	"strings"
	"net/http"
	"io/ioutil"
)

func main() {
	url := "{{.URL}}"
	method := "{{.Method}}"

	payload := strings.NewReader("")

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		fmt.Println(err)
	}

	req.Header.Add("Accept", "{{.Headers.Accept}}")
	req.Header.Add("Referer", "{{.Headers.Referer}}")
	req.Header.Add("User-Agent", "{{.Headers.UserAgent}}")

	res, err := client.Do(req)
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	fmt.Println(string(body))
}
`

	buf := &bytes.Buffer{}
	t := template.Must(template.New("tpl").Parse(goTemplate))
	if err := t.Execute(buf, data); err != nil {
		panic(err)
	}
	body := buf.String()

	err := ioutil.WriteFile(resultGoFile, []byte(body), 0644)
	if err != nil {
		panic(err)
	}

	bufPy := &bytes.Buffer{}
	tt := template.Must(template.New("pytpl").Parse(pyTemplate))
	if err := tt.Execute(bufPy, data); err != nil {
		panic(err)
	}
	pyBody := bufPy.String()

	errPy := ioutil.WriteFile(resultPyFile, []byte(pyBody), 0644)
	if errPy != nil {
		panic(err)
	}

	return

}

// PrettyPrint prints something nice
func PrettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

// PrintBlock help to decorate a comment block when printing in log
func PrintBlock(msg string, format string) {
	separator := " " + strings.Repeat(format, len(msg)+4) + " "
	body := " " + format + " " + msg + " " + format + " "

	// Print
	fmt.Println("")
	fmt.Println(separator)
	fmt.Println(body)
	fmt.Println(separator)

}

package factotum

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// ReqHead as request header for the captured GET requests
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

// Run actualls runs the crawler with headless mode, wait til timeout
// and capture the GET requests during the time with chrome devtool protocol
// jsonOnly will return json output only, otherwise will also return python/go sample program for the GET requests
func Run(ctx context.Context, url string, timeout int, verbose, jsonOnly bool) error {

	// Output folder
	resultFolder, err := getResultFolder(url)
	if err != nil {
		return err
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", true),
		chromedp.Flag("ignore-cerificate-errors", true),
		// chromedp.UserDataDir(dir),
	)

	// Create context
	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	taskCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	taskCtx, cancel = context.WithTimeout(taskCtx, time.Duration(timeout)*time.Second)
	defer cancel()

	if err := chromedp.Run(taskCtx); err != nil {
		return err
	}

	// listen network event with chrome devtools protocol
	fmt.Printf("Listening for network events (XHR) - %d seconds \n", timeout)
	anotherListenFunc(taskCtx, resultFolder, verbose, jsonOnly)

	chromedp.Run(taskCtx,
		network.Enable(),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.BySearch),
	)

	// Print sample program as well, if jsonOnly is false
	if !jsonOnly {
		select {
		case <-time.After(time.Duration(timeout) * time.Second):
			// Print Get program
			fmt.Println("")
			fmt.Println("Post processing - Writing Get reqeust to Go/Python files")
			fmt.Println("")

			files, err := ioutil.ReadDir(resultFolder)
			if err != nil {
				log.Fatal(err)
			}

			for _, file := range files {
				if strings.Contains(file.Name(), "GET") {
					f := path.Join(resultFolder, file.Name())
					PrintGetReq(f) // print go/python programs
				}
			}

			fmt.Printf("Save to %s\\main.go.xxx \n", resultFolder)
		}
	}
	return nil
}

// anotherListenFunc listens for the network event from devtools protocol
// and listen to xhr events only
func anotherListenFunc(ctx context.Context, folder string, verbose, jsonOnly bool) {

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
				cType := ev.Response.Headers["Content-Type"]
				if cType == nil {
					cType = ev.Response.Headers["content-type"]
				}
				contentType, ok := cType.(string) // Parse interface to string
				if !ok {
					return
				}

				if !strings.Contains(contentType, "application/json") {
					return
				}

				go func() {
					// print response body
					c := chromedp.FromContext(ctx)
					rbp := network.GetResponseBody(ev.RequestID)
					body, err := rbp.Do(cdp.WithExecutor(ctx, c.Target))
					if err != nil {
						// fmt.Println(err)
					}
					if string(body) != "{}" {

						filename := filepath.Join(folder, "RESULT-"+ev.RequestID.String())
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
						filename := filepath.Join(folder, method+"-"+ev.RequestID.String())
						req := PrettyPrint(ev.Request)

						if err := ioutil.WriteFile(filename, []byte(req), 0644); err != nil {
							log.Fatal(err)
						}
					}

				}
			}
		})
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

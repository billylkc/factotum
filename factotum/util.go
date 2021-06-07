package factotum

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// PrettyPrint prints something nice
func PrettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

// getResultFolder creates the result folder if not exists
// Return the absolute path, with the domain + first part of url as folder name
// url    - "https://hk.centanet.com/estate/%E5%A4%AA%E5%8F%A4%E5%9F%8E/3-OVDUURFSRJ"
// folder - hk_centanet_com/estate
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

package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"io/ioutil"
	"os"
	"strings"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	uaaclient "code.cloudfoundry.org/uaa-go-client"
	"code.cloudfoundry.org/uaa-go-client/config"
	"code.cloudfoundry.org/uaa-go-client/schema"

	"github.com/cloudfoundry-community/go-cfenv"
)

const (
	// DefaultPort is the port in which to listen if there is no PORT declared
	ServiceUrl = "http://www.google.com/sap/opu/odata/sap/SERVICE_SRV"
	ServiceAuth = "Basic VXNlcjpQYXNzd29yZA=="
	DefaultPort = "9000"
)

// HelloServer is the base funtion to expose text
func HelloServer(w http.ResponseWriter, req *http.Request) {

	var (
		err         error
		uaaClient   uaaclient.Client
		token       *schema.Token
		proxyString string
	)

	// In the cloud foundry environment, CF settings should be parsed
	if strings.Contains(strings.Join(os.Environ(), " "), "VCAP_APPLICATION") {

		appEnv, _ := cfenv.Current()

		// Check for connectivity service
		connectivity, err := appEnv.Services.WithName("oncemore_Development_BMSAE")
		if err != nil {
			fmt.Fprintln(w, "\nConnectivity: false")
		} else {
			proxyString = "http://" + connectivity.Credentials["onpremise_proxy_host"].(string) + ":" + connectivity.Credentials["onpremise_proxy_port"].(string)
		}

		// Check for XSUAA and generate a proxy credentials
		xsuaa, err := appEnv.Services.WithName("oncemore_Development_sneQo")
		if err != nil {
			fmt.Fprintln(w, "\nXSUAA: false")
		}
		cfg := &config.Config{
			ClientName:       connectivity.Credentials["clientid"].(string),
			ClientSecret:     connectivity.Credentials["clientsecret"].(string),
			UaaEndpoint:      xsuaa.Credentials["url"].(string),
			SkipVerification: true,
		}
		logger := lager.NewLogger("test")
		clock := clock.NewClock()
		uaaClient, err = uaaclient.NewClient(logger, cfg, clock)
		if err != nil {
			fmt.Fprintln(w, "Error: ", err)
		}
		token, err = uaaClient.FetchToken(true)
		if err != nil {
			fmt.Fprintln(w, "Error: ", err)
		}

	}

	// Set the proxy if there is one
	var client = &http.Client{}
	if proxyString != "" {
		proxyURL, err := url.Parse(proxyString)
	  if err != nil {
	    fmt.Fprintln(w, "Error: ", err)
		} else {
			client.Transport = &http.Transport{
	      Proxy: http.ProxyURL(proxyURL),
	    }
		}
	}

	// Calling the service and returning the response
	url, err := url.Parse(ServiceUrl + req.URL.Path)
  if err != nil {
		fmt.Fprintln(w, "Error: ", err)
  } else {
		request, err := http.NewRequest("GET", url.String(), nil)
		if err != nil {
			fmt.Fprintln(w, "Error: ", err)
		} else {
			if token != nil {
				request.Header.Add("Proxy-Authorization", "Bearer " + token.AccessToken)
			}
			request.Header.Add("Authorization", ServiceAuth)
			request.Header.Add("Accept", req.Header.Get("Accept"))
			response, err := client.Do(request)
			if err != nil {
					fmt.Fprintln(w, "Error: ", err)
			} else {
				w.Header().Set("Content-Type", response.Header.Get("Content-Type"))
		    data, err := ioutil.ReadAll(response.Body)
		    if err != nil {
		    	fmt.Fprintln(w, "Error: ", err)
		    }
				fmt.Fprintln(w, string(data))
			}
		}
  }
}

func main() {
	var port string
	if port = os.Getenv("PORT"); len(port) == 0 {
		log.Printf("Warning, PORT not set. Defaulting to %+v\n", DefaultPort)
		port = DefaultPort
	}

	http.HandleFunc("/", HelloServer)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Printf("ListenAndServe: %v\n", err)
	}
}

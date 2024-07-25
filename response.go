package req

import (
	"encoding/json"
	"net/http"
)

}

func Do[T any](api Api) (zero T, err error) {
	var client *http.Client
	jar, isJar := api.(http.CookieJar)
	if isJar {
		client = &http.Client{Jar: jar}
	} else {
		client = http.DefaultClient
	}

	req, err := NewRequest(api)
	if err != nil {
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&zero)
	return
}

var Debug = Do[map[string]any]

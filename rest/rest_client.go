package rest
import (
	"net/http"
	"encoding/json"
	"bytes"
	"fmt"
	"os"
	"errors"
	"strings"
	"github.com/pivotal-sydney/whiteboardbot/model"
	"io/ioutil"
)

type RestClient interface {
	Post(request model.WhiteboardRequest, standupId int) (itemId string, ok bool)
	GetStandupItems(standupId int) (items model.StandupItems, ok bool)
	GetStandup(standupId int) (standup model.Standup, ok bool)
}

type RealRestClient struct{}

func (RealRestClient) Post(request model.WhiteboardRequest, standupId int) (itemId string, ok bool) {
	json, _ := json.Marshal(request)
	fmt.Printf("Posting entry to whiteboard:\n%v\n", string(json))
	http.DefaultClient.CheckRedirect = noRedirect
	url := os.Getenv("WB_HOST_URL")
	if len(request.Id) > 0 {
		url += "/items/" + request.Id
	} else {
		url += fmt.Sprintf("/standups/%v/items", standupId)
	}
	httpRequest, err := http.NewRequest(toHttpVerb(request.Method), url, bytes.NewReader(json))
	httpRequest.Header.Add("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(httpRequest)
	fmt.Printf("Whiteboard Request: %v\n\n", httpRequest)
	fmt.Printf("Whiteboard Response: %v, Err: %v\n, Url: %v\n\n", resp, err, url)

	ok = resp !=nil && resp.StatusCode == http.StatusFound
	defer resp.Body.Close()
	if ok {
		itemId = resp.Header.Get("Item-Id")
	}
	if (len(itemId) == 0) {
		itemId = request.Id
	}
	return
}

func (RealRestClient) GetStandupItems(standupId int) (items model.StandupItems, ok bool) {
	url := fmt.Sprintf("%v/standups/%v/items", os.Getenv("WB_HOST_URL"), standupId)
	httpRequest, _ := http.NewRequest("GET", url, nil)
	httpRequest.Header.Add("Accept", "application/json")
	resp, err := http.DefaultClient.Do(httpRequest)
	defer resp.Body.Close()
	ok = err == nil && resp != nil && resp.StatusCode == http.StatusOK

	if ok {
		jsonBlob, err := ioutil.ReadAll(resp.Body)
		ok = err == nil
		if ok {
			err = json.Unmarshal(jsonBlob, &items)
			ok = err == nil
		}
	}
	return
}

func (RealRestClient) GetStandup(standupId int) (standup model.Standup, ok bool) {
	url := fmt.Sprintf("%v/standups/%v", os.Getenv("WB_HOST_URL"), standupId)
	httpRequest, _ := http.NewRequest("GET", url, nil)
	httpRequest.Header.Add("Accept", "application/json")
	resp, err := http.DefaultClient.Do(httpRequest)
	defer resp.Body.Close()
	ok = err == nil && resp != nil && resp.StatusCode == http.StatusOK

	if ok {
		jsonBlob, err := ioutil.ReadAll(resp.Body)
		ok = err == nil
		if ok {
			err = json.Unmarshal(jsonBlob, &standup)
			ok = err == nil
		}
	}
	return
}


func noRedirect(req *http.Request, via []*http.Request) error {
	return errors.New("Don't redirect!")
}

func toHttpVerb(method string) (httpVerb string) {
	if len(method) > 0 {
		httpVerb = strings.ToUpper(method)
	} else {
		httpVerb = "POST"
	}
	return
}
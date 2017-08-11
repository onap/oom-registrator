package restclient

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

type RESTClient struct {
	base     string
	resource string
	param    string
}

func NewRESTClient(baseURL string, versionedAPIPath string, urlParameter string) *RESTClient {
	return &RESTClient{
		base:     baseURL,
		resource: versionedAPIPath,
		param:    urlParameter,
	}
}

func (c *RESTClient) Get() (b []byte, err error) {
	url := c.base + "/" + c.resource + "/" + c.param
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf(res.Status)
	}

	buf, err := ioutil.ReadAll(res.Body)
	res.Body.Close()

	if err != nil {
		return nil, err
	}

	return buf, nil
}

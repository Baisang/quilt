package stitch

import (
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/robertkrimen/otto"
)

// HTTPGet is the function used to make the HTTP GET request for the GitHub keys.
// Exported so that we can run specs in tests without actually interacting
// with the network.
var HTTPGet = http.Get

var githubCache = make(map[string][]string)

func githubKeys(username string) ([]string, error) {
	if keys, ok := githubCache[username]; ok {
		return keys, nil
	}
	keys, err := getGithubKeys("https://github.com/" + username + ".keys")
	if err != nil {
		return nil, err
	}
	githubCache[username] = keys
	return keys, nil
}

func getGithubKeys(keyURL string) ([]string, error) {
	res, err := HTTPGet(keyURL)
	if err != nil {
		return nil, err
	}
	keyBytes, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		return nil, err
	}
	keys := strings.TrimSpace(string(keyBytes))
	keyStrings := strings.Split(keys, "\n")
	return keyStrings, nil
}

func githubKeysImpl(call otto.FunctionCall) (otto.Value, error) {
	if len(call.ArgumentList) < 1 {
		panic(call.Otto.MakeRangeError(
			"githubKeys requires the username as an argument"))
	}

	username, err := call.Argument(0).ToString()
	if err != nil {
		return otto.Value{}, err
	}

	keys, err := githubKeys(username)
	if err != nil {
		return otto.Value{}, err
	}

	keysVal, err := call.Otto.ToValue(keys)
	if err != nil {
		return otto.Value{}, err
	}

	return keysVal, nil
}
